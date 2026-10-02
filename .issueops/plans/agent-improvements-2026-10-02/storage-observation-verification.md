# I8 저장 관찰 · WithRecordGuard seam 재검증 (보정 후)

검증 시각: 2026-10-02T10:49Z 이후 / 기준 HEAD `92eaa001` (작업 트리 변경분).
코드, 테스트, 설정, inventory는 수정하지 않았다. 모든 반증 실험은 `/tmp`의 `go test -overlay`로 했고,
저장소에는 `zz_` 파일이 없다(`git status` 확인).

검증자 세션 모델: `claude-sonnet-5-5` (`tool.bash`의 `printenv PI_MODEL`, 이 세션에서 실행).
effort는 사용자 정정에 따라 조사하지 않았다. producer 문서가 기록한 `claude-opus-5-5`와 모델이 다르다.

읽은 것: `contract.md`(§I8, §원자성), `implementation-storage-observation.md`, 이전 검증 문서,
`sqlstore.go` diff 전체, `span_observer.go`, `record_guard.go`, `issueopsrecord/store.go` diff,
`span_coverage_test.go`, `span_observer_test.go` 앞부분.

## 1. 요약

- 이전 검증에서 지적한 항목 중 코드가 바뀐 것(CAS/Delete 테스트 부재, baseline 경합 창, commit 직전 취소, span 도중 가시성)은
  모두 소스와 실행으로 확인했다. 새 소스 결함은 찾지 못했다.
- 반증 실험에서 **테스트 빈틈 1건**(start epoch 증가를 고정하는 테스트 없음, 변이 M7 생존)과
  **테스트 견고성 문제 1건**(hook 대기에 시한이 없어 회귀 시 timeout까지 멈춤, 변이 M4)을 새로 찾았다.
- 미해결 결정은 이전과 같다: guard 1회 소비, seam 미사용(production 호출자 없음), 성능 baseline 없음.

## 2. 실행 결과 (이 세션에서 직접 실행)

| 명령 | 결과 |
|---|---|
| `go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/issueopsrecord -count=1` | exit 0: 5.608s / 1.139s |
| `go test -race` 같은 두 패키지 `-count=1` | exit 0: 9.839s / 2.293s |
| `gofmt -l` (두 패키지) | 출력 없음, exit 0 |
| `go vet` (두 패키지) | exit 0 |
| `git diff --check -- sqlstore issueopsrecord` | exit 0 |
| `grep -rn WithRecordGuard --include=*.go` (sqlstore 밖) | 일치 없음(exit 1): production 호출자 없음 |

architecture inventory 테스트는 이번에 다시 돌리지 않았다(범위 밖이며 producer가 다른 owner drift를 보고함). 미검증으로 둔다.

### 실제 SQLite 증거 (producer 시나리오 파일 `/tmp/i8-evidence/zz_i8_evidence_test.go` 그대로 재실행)

```sh
go test -overlay /tmp/i8-evidence/overlay.json ./internal/adapter/outbound/issueopsrecord -run TestI8Evidence -count=1 -v
```

시나리오 파일은 `Store.Update`와 `NewJSONLineObserver`를 실제 임시 root에서 쓴다. 출력(요약, 값은 이번 실행):

| # | 상황 | outcome | wait/hold/callback ms | commit_ms | count | coverage |
|---|---|---|---|---|---|---|
| 1 | 120ms 느린 callback | success | 0/123/123 | 0 | 1 | complete |
| 2 | 60ms holder 뒤 대기(contended=true) | success | 62/0/0 | 0 | 1 | complete |
| 3 | callback 안의 context 없는 `DB.Put` | success | 0/1/1 | **null** | 1 | **unknown** |
| 4 | 평범한 갱신 | success | 0/1/1 | 0 | 1 | complete |
| 5 | callback 안에서 취소 | canceled | 0/0/0 | 0 | **0** | complete |

3행(null/unknown)과 4행(측정된 0/complete)이 구분된다. 5행은 취소된 요청이 Commit을 부르지 않았다는 실행 증거다.
한계: 실제 시계로 잰 `commit_ms`는 전부 sub-ms라 ms floor로 0이다. 0이 아닌 commit 시간은 scripted clock 테스트에서만 확인된다.

### Benchmark (Apple M1 Pro, 단독 실행, `-count=5`)

| benchmark | ns/op 5회 | B/op | allocs/op |
|---|---|---|---|
| sqlstore empty/nil | 11479, 11592, 11489, 11379, 11567 | 1616-1618 | 21 |
| sqlstore empty/noop | 12232, 12213, 11677, 12355, 11784 | 1632-1633 | 23 |
| sqlstore one_commit/nil | 208582, 215491, 229343, 248768, 231329 | 3283-3284 | 54 |
| sqlstore one_commit/noop | 235606, 225784, 233609, 270710, 262266 | 3298-3299 | 56 |
| store Update nil | 288320, 346910, 294651, 316681, 267030 | 11789-11885 | 128 |
| store Update noop | 264559, 271577, 278649, 322438, 266260 | 12049-12084 | 135 |
| store Update json_actionable | 295716, 306084, 314454, 329023, 333881 | 12653-12734 | 138 |

allocs/op는 producer가 보고한 값(21/23/54/56/128/135/138)과 모두 같다. ns/op는 SQLite I/O 분산이 커서
observer 비용을 판정할 수 없다(Store nil과 noop의 순서도 뒤집힌다). 변경 전 baseline은 없고, producer도 측정하지 못했다.

## 3. 반증 실험

### 3.1 producer 테스트 스위트에 대한 변이 (`sqlstore.go`만 overlay로 바꾸고 두 패키지 전체 실행)

| 변이 | 결과 |
|---|---|
| M1a `Apply`가 `commitData` 대신 `tx.Commit()` | 잡힘: 9개 이상 테스트 실패(출력을 12줄로 잘랐다) |
| M1b `CompareAndApplyFunc`가 `tx.Commit()` | 잡힘: `TestSpanCompareAndApplyCommitIsAttributed` |
| M2 `Delete` 추적 제거 | 잡힘: `TestSpanAutocommitDeletesAreUnknown` |
| M3 `DeleteBucket` 추적 제거 | 잡힘: 같은 테스트 |
| M4 `Put` 추적 제거 | **assertion이 아니라 멈춤으로 잡힘**: 테스트가 `<-started`에서 영구 대기해 `go test`가 자동 detach 시점(약 60초)을 넘겨서도 끝나지 않아 수동 종료했다(아래 4.2) |
| M5 baseline in-flight 표본 제거 | 잡힘: `TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline` |
| M6 최종 epoch 비교 제거 | 잡힘: `TestSpanAutocommitDeletesAreUnknown`, `TestSpanCommitCoverage` |
| **M7 write 시작 시 epoch 증가 제거** | **생존: 두 패키지 모두 통과** |
| M8 commit 직전 `ctx.Err()` 확인 제거 | 잡힘: `TestSpanCommitChecksCancellationBeforeCommit` |
| M9 span 밖 Apply의 `unattributedWrite` 제거 | 잡힘: `TestSpanCommitCoverage` |
| M10 실패한 Commit을 count에서 제외 | 잡힘: `TestSpanCommitCoverage` |

### 3.2 내가 쓴 overlay 테스트 (현재 코드에서 모두 통과)

`/tmp/verify-i8b/zz_verify_test.go`, `zz_verify2_test.go`. 테스트 8개(아래 7개 시나리오 + 3.3)가 현재 코드에서 모두 통과했다.

- CAS + Delete mutation: count 1/complete, 행 삭제됨. 오래된 expected로 CAS 실패(`RawCASError`): outcome error, count 0, Commit 측정된 0, 행 유지.
- commit 직전 취소(CAS 경로): `context.Canceled`, 행이 삭제되지 않음, count 0, outcome canceled.
- span 안의 `Mutate`(`Apply(context.Background())`): commit nil/unknown.
- span 이전에 끝난 write: complete, count 0(불필요한 unknown 없음).
- span 안에서 시작해 반환 뒤 commit되는 write: unknown(보수적).
- binder panic: callback 미호출, outcome error, Callback nil, 이후 span이 lock을 정상 획득한다. binder가 `context.Background()`를 반환하면 callback과 write 없이 오류.
- 같은 root의 다른 `*DB`로 commit(아래 4.3): `count=0 coverage=complete`로 보고된다.

### 3.3 M7 반증 테스트 (현재 코드 통과, M7 변이 실패)

시나리오: baseline **이후**에 시작한 귀속 불가 `Put`이 hold 중에 commit되고, 그 끝 기록만 span 반환 뒤로 미뤄지는 경우.

```text
CURRENT CODE: PASS
M7 MUTANT:    FAIL  ... write that started after the baseline and committed during the hold was reported complete:
              {Outcome:success ... Commit:0s CommitCount:0 CommitCoverage:complete}
```

테스트 구조: `openSharedRoot`의 두 번째 핸들이 data DB write lock을 잡는다. span callback 안에서 goroutine이 `Put`을 시작하고
`unattributedWriteStarted`를 기다린다. callback이 lock을 풀고 `unattributedWriteFinished`(끝 기록 전, `<-resume`으로 막음)를 기다린 뒤 반환한다.
span 반환 뒤에 `resume`을 닫고 관측값이 unknown인지 검사한다. 이 테스트는 producer 스위트에 없다.

## 4. 이전 검증 항목별 재판정

| 이전 항목 | 판정 | 근거 |
|---|---|---|
| 1. CAS/Delete/DeleteBucket commit 귀속 테스트 부재(M1-M3 생존) | **해소** | M1b, M2, M3 모두 잡힘. 내 CAS+Delete와 stale CAS 테스트도 통과. |
| 2. baseline 이전 시작 write 경합 창 | **소스 해소, 테스트는 일부 빈틈** | `sqlstore.go`가 baseline(epoch→in-flight)을 lock 획득 전에, 비교를 rollback 뒤에 한다. in-flight 표본(M5)과 최종 비교(M6)는 테스트가 잡는다. start epoch 증가(M7)는 안 잡는다. |
| 3. guard 1회 소비 | **미해결 결정** | 소스 그대로(`recordGuardFunc(nil)`로 덮어씀). producer가 G에게 넘겼다. |
| 4. seam production 호출자 없음 | **그대로** | 호출자 검색 결과 없음. end-to-end 증거는 sqlstore 단위 테스트뿐이다. |
| 5. commit 직전 취소 명시 확인 부재 | **해소** | `commitData`의 `ctx.Err()` 확인, M8 잡힘, CAS 경로에서도 내 테스트로 확인. commit 뒤 취소는 `TestSpanCancellationAfterCommitKeepsData`. |
| 6. 성능 baseline 부재 | **그대로** | 변경 전 측정 없음. allocs/op만 안정적이다. |
| 7. inventory drift | **재검증 안 함** | 이번 범위 밖. |
| 개별 write 가시성(실행 증거 부분적) | **해소** | `TestSpanApplyIsVisibleToAnotherHandleBeforeSpanExit`가 span lock 보유 중에 다른 핸들에서 upsert/delete를 읽는다. M1a가 이 테스트를 깨뜨린다. |

### 4.1 주장별 최종 판정

| 주장 | 판정 | 근거 |
|---|---|---|
| commit은 실제 data commit이며 lock rollback이 아니다 | 확인 | `commitData`가 `tx.Commit()`만 감싼다(소스). M1a/M1b가 실패하고, 증거 5행에서 취소 시 count 0. |
| unknown과 0 구분 | 확인 | 증거 3, 4행. `unattributedWrites`일 때만 nil/unknown, 아니면 0 포함 complete. |
| unlock 뒤 재진입 | 확인 | observer는 `runSpan` 반환(gate와 lock 해제) 뒤에 호출된다(소스). 재진입 테스트 통과. |
| 취소 | 확인 | gate 대기 중 취소, commit 전 취소(명시 확인), commit 뒤 취소(데이터 보존). commit 호출 도중 취소는 아래 4.4. |
| panic | 확인 | `returned` 플래그로 error 처리 후 재전파. binder panic도 내 테스트에서 lock 해제 확인. |
| nested/concurrent 격리 | 확인 | race 통과. 단 `activeSpanRun`은 `*DB` 포인터로 격리하므로 다른 핸들은 보이지 않는다(4.3). |
| 개별 write 가시성 | 확인 | 위 표. |
| binder 실패 시 callback/write 없음, nested span 없음 | 확인 | binder panic, 실패, 잘못된 context 반환 모두 callback 미호출/행 없음. 기존 테스트는 같은 root span의 `NestedSpanError`를 확인한다. |

### 4.2 결함: hook 대기에 시한이 없다 (테스트 견고성, 중간)

`TestSpanCoverageSeesUnattributedWriteInFlightAtBaseline`는 `<-started`, `<-committed`를 시한 없이 기다린다(소스 확인).
M4 실행이 멈춘 것은 이 대기 때문이라고 판단하나, 어느 테스트에서 멈췄는지는 로그를 받기 전에 종료해 확인하지 못했다. `Put`이 `unattributedWrite`를 거치지 않는 회귀(M4)가 들어오면 assertion 실패가 아니라
`go test` 기본 timeout까지 멈춘다. 이 저장소의 테스트 규칙(신호를 bounded timeout으로 기다림)과 어긋난다.
조치: hook 채널 대기를 `select`와 `time.After` 같은 상한이 있는 대기로 바꾼다.

### 4.3 결함: 다른 `*DB` 핸들의 commit은 이 span이 보지 못한다 (낮음, 문서 필요)

`activeSpanRun`은 `*DB` 포인터 일치로 run을 찾는다. 같은 root의 두 번째 핸들(테스트 전용 `newDB`)이 span context로 `Apply`하면
데이터는 commit되지만 span 관측은 `commit_count=0, coverage=complete, commit_ms=0`이다. 내 실험으로 확인했다.
production `Open`은 절대 경로별로 핸들을 캐시하므로(`sqlstore.go`의 `handles`) 같은 프로세스에서는 일어나지 않고,
다른 프로세스의 write도 같은 방식으로 보이지 않는다. producer 문서는 이를 "한계"로 적었으므로 소스와 문서는 일치한다.
다만 complete가 "이 프로세스 핸들의 write만 완전함"이라는 뜻임을 contract에 한 줄로 명시하는 편이 안전하다.

### 4.4 소스로만 확인한 항목 (실행하지 않음)

- `ctx.Err()` 확인과 `tx.Commit()` 사이에 취소가 도착하는 경우: `BeginTx(ctx)`의 database/sql 취소 처리에 의존한다.
  이 구간에 끼워 넣을 hook이 없어 결정적으로 재현하지 못했다. 결과는 Commit 실패(count는 증가, 문서화된 동작)일 것이다.
- span이 반환된 뒤에도 span context로 commit하는 goroutine이 남는 경우: `active`가 false가 된 뒤 도착한 commit은 귀속 불가 write가 되고,
  `active` 확인 직후 span이 끝나면 관측이 합계를 놓칠 수 있다. producer가 한계로 적었고 실행하지 않았다.
- baseline을 lock 획득 **전에** 뜨는 순서의 이유: 획득 직후에 표본을 뜨면 획득과 표본 사이에 commit된 write를 놓친다. 순서는 맞다. 소스 판독이다.

## 5. 남은 항목 (수정하지 않음)

1. **테스트 빈틈**: start epoch 증가(M7)를 고정하는 테스트가 없다. 3.3의 테스트 구조를 `span_coverage_test.go`에 추가하면 된다.
2. **테스트 견고성**: hook 대기에 시한이 없다(4.2).
3. **문서 보강**: coverage `complete`의 범위(같은 `*DB` 핸들만)를 contract에 명시한다(4.3).
4. **미해결 결정**: guard 1회 소비는 G가, seam 사용처는 G/M이 정해야 한다.
5. **미측정**: 변경 전 성능 baseline, architecture inventory 현황, 4.4의 소스 전용 항목.

이 문서는 producer 작업을 승인하지 않는다. 위 1번은 완료 승인 전에 메우는 것을 권한다. 소스 결함은 이번에 찾지 못했다.
