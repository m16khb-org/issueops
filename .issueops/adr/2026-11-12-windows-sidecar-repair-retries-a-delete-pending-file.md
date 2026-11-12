---
name: 2026-11-12-windows-sidecar-repair-retries-a-delete-pending-file
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Windows sidecar repair retries a delete-pending file

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session (round 3, WS-A `ws-sqlrace`)
- Summary: Windows에서 `repairPrivateSQLiteFiles`는 `-wal`/`-shm`/`-journal` sidecar의 `ERROR_ACCESS_DENIED`만 호출당 500ms sleep 예산 안에서 Lstat부터 다시 시도하고, 예산을 다 쓰면 원래 오류로 실패한다. main DB 파일과 Unix는 재시도하지 않는다.
- Context: `TestStartIssueOpsChildConcurrentSiblingsAcrossProcesses`와 `TestAcceptIssueOpsChildrenConcurrentlyAcrossProcesses`가 Windows에서 192회 중 1회, 3회 `sqlstore inspect permissions ...-wal: CreateFile ...: Access is denied.` 또는 `sqlstore chmod ...: Access is denied.`로 실패했다. Windows의 sqlstore는 `SetMaxIdleConns(0)`이라 연산마다 마지막 연결을 닫고, SQLite는 그때 `-wal`/`-shm`을 지운다. 다른 프로세스가 아직 그 파일을 열고 있으면 이름은 delete-pending으로 남고, `GetFileAttributesEx`·`CreateFile`·`GetFileAttributes`가 not-found가 아니라 `ERROR_ACCESS_DENIED`를 돌려준다. `repairPrivateSQLiteFiles`(`newDB`가 두 번, `Open`과 `Maintain`이 한 번씩 부른다)는 이 오류를 치명적으로 다뤘다. cautions/runtime.md의 "permission, symlink, schema, path 오류를 retry로 숨기지 않는다" 규칙이 이 재시도를 막으므로 범위를 정한 예외로 기록한다.
- Decision: 1) 파일 하나의 순서(Lstat → regular-file 확인 → Perm → Chmod)를 `repairPrivateSQLiteFile`로 분리한다. 2) sidecar(`sqliteFileSuffixes`에서 빈 suffix를 뺀 것)에서 `isTransientSidecarError`가 참일 때만 그 순서를 Lstat부터 다시 실행한다. Windows 구현은 `errors.Is(err, windows.ERROR_ACCESS_DENIED)`, Unix 구현은 항상 false다. 3) 대기는 1ms에서 두 배씩 늘어 50ms에서 멈추고, 요청한 sleep의 합계는 `sidecarRetryBound`(500ms)에서 멈춘다(마지막 간격을 잘라 맞춘다). 이것은 wall time이 아니라 sleep 예산이며, 시도마다의 Lstat/ACL 호출 시간은 그 위에 더해진다. 예산은 파일마다가 아니라 `repairPrivateSQLiteFiles` 호출 하나에 하나다. `Open`은 캐시 여부와 관계없이 매 호출마다 `handlesMu`를 쥔 채 repair를 한 번 부르고, 캐시가 없으면 `newDBWithRetry`로 넘어간다. `newDB` 시도 하나는 repair를 최대 두 번(시작할 때, schema 초기화 뒤) 부르고, `newDBWithRetry`는 `SQLITE_BUSY`/`SQLITE_LOCKED` 뒤 `openLockMaxWait`(10초)까지 다시 시도하되 deadline을 시도 사이에서만 본다. 그래서 deadline 직전에 시작한 시도는 repair sleep만으로 약 1초 더 갈 수 있고(데이터 DB의 `busy_timeout` 대기는 별도다), 이 모두가 `handlesMu` 아래에서 일어나므로 예산을 짧게 둔다. 4) 예산을 다 쓰면 마지막 시도의 오류(`sqlstore inspect permissions` 또는 `sqlstore chmod`로 감싼 원래 오류)를 그대로 돌려준다. 5) main DB 파일(`issueops.db`, `issueops.lock.db`)은 재시도하지 않는다. SQLite는 정상 동작 중 이 파일을 지우지 않으므로(span DB는 rollback-journal 모드라 sidecar가 `-journal`뿐이다) 그 access-denied는 실제 오류다. 6) regular file이 아닌 sidecar와 다른 원인의 Chmod 실패는 지금처럼 즉시 실패한다. 7) sleep은 패키지 변수 `sidecarRetrySleep`으로 주입해 테스트가 기다리지 않고 호출을 센다.
- Consequences: delete-pending sidecar 때문에 Open이 실패하지 않는다. 실제로 DACL이 막은 sidecar는 즉시가 아니라 repair 호출마다 500ms sleep 예산을 다 쓴 뒤 같은 오류로 실패한다. `Open`은 그동안 `handlesMu`를 쥐고 있어 같은 프로세스의 다른 `Open`도 그만큼 기다린다. 캐시된 root의 `Open`은 repair 한 번분, 처음 여는 root는 거기에 `newDB` 시도마다 두 번분이 더해지며, 마지막 시도는 10초 open deadline을 약 1초 넘길 수 있다. 2026-07-08 ADR의 "고정된 main/sidecar 집합을 0600으로 고친 뒤 반환한다"는 그대로다. Unix 경로는 sleep하지도 반복하지도 않는다.
- Evidence:
  - 조사 probe(배포하지 않음): classic disposition으로 delete-pending인 `-wal`에 `os.Lstat`이 `ERROR_ACCESS_DENIED`를 내고, 실제 SQLite churn에서 표본으로 뜬 모든 오류의 NT status가 `0xC0000056`(`STATUS_DELETE_PENDING`)이었다. 같은 사용자의 동시 Chmod(39,813회, 오류 0)와 SQLite 공유 모드(오류 32가 됐을 것)는 원인에서 제외했다.
  - 스트레스 비교(2026-11-12, Windows, runner 8개 × `-count=36` × 두 cross-process 테스트 = 576회): 기준 `25906292a`는 12회 실패했고 실패 문구는 `sqlstore inspect permissions`(4), `sqlstore chmod`(4), `workers did not reach ready barrier`(4)뿐이었다. 수정본은 576회 모두 통과했다.
  - internal/adapter/outbound/sqlstore/sqlstore.go (`repairPrivateSQLiteFiles`, `repairPrivateSQLiteFile`, `sidecarRetry`)
  - internal/adapter/outbound/sqlstore/sqlite_sidecar_windows.go, sqlite_sidecar_unix.go
  - internal/adapter/outbound/sqlstore/sidecar_retry_windows_test.go: 놓아준 delete-pending `-wal`은 sleep 한 번 뒤 통과하고, 넓은 ACL로 다시 생긴 sidecar는 0600으로 고쳐 보고하며, `-wal`과 `-shm`이 함께 막히면 두 파일이 sleep 예산 하나를 나눠 쓰고 요청 합계가 정확히 500ms가 된 뒤 `-shm`의 `ERROR_ACCESS_DENIED`로 실패하고(예산을 파일마다 두거나 마지막 간격을 자르지 않는 변형에서 이 테스트가 실패함을 확인했다), Open은 handle을 캐시하지 않으며, delete-pending main DB는 sleep 없이 실패한다.
  - internal/adapter/outbound/sqlstore/sidecar_retry_test.go: 디렉터리 sidecar는 재시도 없이 거부되고, Windows 밖에서는 errno 5가 transient가 아니다.
- Alternatives / rejected options:
  - access-denied면 재시도 없이 건너뛰기: 오류 5는 실제 DACL 거부도 돌려주는 값이고, 2026-07-08 ADR은 반환 전에 고정 집합을 0600으로 고치도록 요구한다. 건너뛰면 권한 문제를 숨긴다.
  - `RtlGetLastNtStatus() == STATUS_DELETE_PENDING` 확인: 정확하지만 thread-local 상태에 기대므로 Go 스케줄러가 호출 사이에 goroutine을 다른 OS thread로 옮기면 틀린 값을 읽는다.
