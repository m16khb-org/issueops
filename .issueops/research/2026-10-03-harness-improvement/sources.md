---
name: sources.md
description: 2026-10-03 하네스 성능·동시성 공식 문서 조사와 3개 개선 과제 제안 (근거·베이스라인·수용 기준 포함).
---

# Harness Improvement Research Sources (2026-10-03)

범위: 공식 외부 문서(Go pprof/diagnostics, Go memory model, SQLite WAL, MCP transports)와
이 저장소의 아키텍처/테스트 문서만 읽고, 본 보고서 하나만 작성했다. 구현·커밋·설치 변경 없음.

## 1. 공식 소스 (읽은 원문 기준)

| 주제 | URL | 핵심 내용 |
|------|-----|-----------|
| Go 진단/프로파일링 | https://go.dev/doc/diagnostics | `runtime/pprof`, `net/http/pprof`가 표준 프로파일링 표면. cpu/heap/goroutine/block/mutex 프로파일 제공. block·mutex는 기본 비활성(`runtime.SetBlockProfileRate`, `runtime.SetMutexProfileFraction` 필요). "production 프로파일링은 안전하되 overhead 측정 선행", "프로파일은 한 번에 하나씩 수집" 권고. |
| Go 메모리 모델 | https://go.dev/ref/mem (2022-06-06 판) | data race는 오류(DRF-SC). 공유 데이터는 채널·`sync`·`sync/atomic`으로 직렬화. race는 `-race`가 보고하고 종료시킬 수 있음. |
| SQLite WAL | https://www.sqlite.org/wal.html (2026-08-25 갱신) | WAL은 readers/writers 비차단. `synchronous=NORMAL`이면 checkpoint만 fsync. 읽기 성능은 WAL 크기에 비례해 저하 → checkpoint가 중요. 동시 reader가 항상 존재하면 checkpoint starvation으로 WAL 무한 성장. **WAL-reset bug**(§11): 3.7.0~3.51.2에서 다중 커넥션이 동시에 write/checkpoint할 때 드물게 corruption 가능, 3.51.3(2026-03-13)에서 수정. WAL 모드 SQLITE_BUSY 예외 사례(§9). |
| MCP transports | https://modelcontextprotocol.io/docs/concepts/transports | 표준 바인딩은 stdio와 Streamable HTTP(단일 endpoint POST, 응답은 JSON 또는 request-scoped SSE). JSON-RPC는 UTF-8 필수, 요청 메타는 body가 source of truth이고 HTTP 헤더로 미러링 가능. |

## 2. 저장소 제약 (검증된 코드 근거)

모두 이번 세션에서 직접 확인:

- SQLite 드라이버: `modernc.org/sqlite v1.53.0` (`go.mod:14`). CHANGELOG 확인 결과 번들 SQLite는 **3.53.2** — WAL-reset 버그 수정판(≥3.51.3)보다 새 3.53.x. `read`: `$(go env GOMODCACHE)/modernc.org/sqlite@v1.53.0/CHANGELOG.md:27`.
- 데이터 DB open DSN: `internal/adapter/outbound/sqlstore/sqlstore.go:223` — `_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate`. span lock은 `sqlstore.go:244`의 별도 `issueops.lock.db`(busy_timeout(0), BEGIN IMMEDIATE). read-only 경로는 `sqlstore.go:688,758`(busy_timeout 가변).
- `.issueops/TECH_STACK.md:101`, `.issueops/adr/decisions/2026-07-07-sqlite-state-storage-migration.md`: WAL + BEGIN IMMEDIATE span 직렬화가 canonical 설계.
- MCP HTTP 서비스: `internal/adapter/mcpservice/service.go`(451 lines), `cmd/issueops/mcpcli/mcp_http.go`(188 lines). 클라이언트 기본 `Timeout: 3s`(service.go:76), 서버 `ReadHeaderTimeout: 10s`(mcp_http.go:25,166). initialize probe가 `Accept: application/json, text/event-stream`을 보내고 SSE/JSON 양쪽을 파싱(service.go:206,227) — Streamable HTTP 바인딩과 일치.
- **프로파일링 표면 부재**: `grep -rn "net/http/pprof|runtime/pprof|expvar" --include='*.go' .` 결과 0건. 장기 상주하는 `mcp service` 프로세스의 heap/goroutine 상태를 관측할 수단이 코드에 없다. `.issueops/ARCHITECTURE.md` runtime 섹션은 장기 실행 프로세스의 stale lock/orphan/메모리 growth 관점을 invariant로 요구.
- HTTP 서비스 신뢰 경계: 같은 OS 사용자 + `issueops mcp authorize`가 발급한 authority_file bearer (`.issueops/ARCHITECTURE.md`, 2026-10-02 ADR). pprof endpoint를 붙인다면 이 경계를 우회하면 안 된다.

### 관찰된 것 vs 가설

| 구분 | 내용 |
|------|------|
| 관찰(측정) | sqlstore 패키지 테스트 1회 16.2s wall / 6.9s user (`time go test ./internal/adapter/outbound/sqlstore/ -count=1`, 이번 세션 실측) — busy_timeout(0) 테스트 DSN(`sqlstore_test.go:87,146`)의 backoff 대기가 원인으로 보이나 프로파일 없이 미확정 |
| 관찰(정적) | pprof/expvar 사용 0건, 번들 SQLite 3.53.2, DSN pragma 문자열 4곳에 하드코딩 |
| 가설(미측정) | 서비스 프로세스의 메모리 성장·goroutine 누수 가능성(stability-audit skill이 우려하는 항목이나 현 시점 측정값 없음) |
| 가설(미측정) | 동시 다수 MCP 클라이언트에서 장기 read 커넥션이 checkpoint를 starve시킬 가능성 — SQLite 문서의 일반 사례이며 issueops 워크로드에서 관찰된 적 없음 |

## 3. 정확한 재현 가능 베이스라인 명령

```bash
# B0 빌드 + 형식/정적 검사 (변경 전 기준선)
go build -o "$(mktemp -d)/issueops" ./cmd/issueops && echo BUILD_OK
gofmt -l $(git ls-files '*.go'); go vet ./...

# B1 SQLite 드라이버가 번들한 SQLite 버전 확인 (WAL-reset ≥3.51.3 가드 대상)
grep -n "Upgrade to \[SQLite" "$(go env GOMODCACHE)/modernc.org/sqlite@$(go list -m -f '{{.Version}}' modernc.org/sqlite)/CHANGELOG.md" | head -3

# B2 sqlstore 동작 확인 (pragma/wal 회귀 테스트가 붙을 자리)
go test ./internal/adapter/outbound/sqlstore/ -count=1

# B3 동시성 회귀 (memory model: race는 오류 — race detector가 보고·종료)
go test -race ./internal/adapter/mcpservice/ ./internal/adapter/outbound/sqlstore/ -count=1

# B4 서비스 프로세스 런타임 관측 (현 시점: 이런 관측 코드가 존재하지 않음이 개선 대상)
./bin/issueops mcp service status --json
GODEBUG=gctrace=1 ./bin/issueops mcp service start   # 임시 외부 관측 수단 (go.dev/doc/diagnostics)
```

## 4. 개선 과제 제안 (T2는 필수, T1은 선택 관측성, T3는 보류)

> 요약: 현재 관찰된 병목은 없다. 유일한 필수 과제는 T2의 동작 검증 테스트이고, T1은 향후 조사를 위한 선택적 관측 인프라, T3는 현 구조에서 오보를 낼 수 있어 보류다.

### T1. `mcp service` HTTP 서버에 opt-in pprof endpoint 추가 — 선택적 관측성 개선 (필수 성능 개선 아님)

- **근거와 한계**: go.dev/doc/diagnostics가 `net/http/pprof`를 production 프로파일링 표준으로 명시. repo에 pprof 사용 0건(§2 grep). 그러나 **pprof 부재 자체는 병목의 증거가 아니다** — 현 시점 관찰된 병목은 없고, 이 과제는 향후 조사(stability-audit의 메모리 성장·goroutine 누수 확인)를 가능하게 하는 관측 인프라일 뿐이다. 우선순위를 상으로 두지 않는다.
- **파일 범위**: `cmd/issueops/mcpcli/mcp_http.go` (서버 mux 조립, 188 lines) + `internal/adapter/mcpservice/service.go` (상수/설정, ≤20 lines).
- **설계 제약**: 기본 off (환경변수 또는 service start 플래그로 opt-in). 활성화 시 기존 bearer/authority 검증을 그대로 통과시키고, 문서상 "프로파일은 한 번에 하나" 권고를 README/OPERATIONS에 반영. block/mutex 프로파일 rate는 활성화 시에만 설정.
- **위험**: endpoint 노출 → 이미 같은 OS 사용자 신뢰 경계 안이므로 낮음. CPU 프로파일 오버헤드 → 기본 off로 회피.
- **수용 기준**: (a) 기본 실행 시 `/debug/pprof/`가 404, (b) opt-in 후 bearer로 `heap`·`goroutine` 프로파일 취득 성공, (c) `go test ./cmd/issueops/mcpcli/ ./internal/adapter/mcpservice/ -count=1` green, (d) B0 baseline 전부 green.

### T2. sqlstore 잠금·내구성 **동작** 검증 테스트 — 우선순위 상

- **근거**: SQLite 공식 문서(§2.2, §9)가 보장하는 것은 동작이다 — "WAL에서 reader와 writer는 서로 비차단", "동시 write 경합은 SQLITE_BUSY", "synchronous=NORMAL이면 checkpoint만 fsync". 현 구현(`sqlstore.go:223,244`)이 이 동작을 실제로 보존하는지는 DSN 문자열이 아니라 관찰 가능한 동작으로 잠가야 한다. 기존 테스트는 DSN을 훼손해도 동작 회귀를 잡는지 검증된 바 없다.
- **파일 범위**: `internal/adapter/outbound/sqlstore/sqlstore_test.go` (새 동작 테스트, 비생산 코드).
- **내용** (모두 observable behavior 단정, 문자열/버전 리터럴 금지):
  1. **경합 직렬화**: 두 커넥션이 동일 state root에 동시 write span을 열 때 하나는 대기하거나(busy_timeout 경로) `SQLITE_BUSY`로 실패하는지, 그리고 span이 프로세스 사망 시 해제되는지(크래시 후 재진입) 관찰.
  2. **WAL 동작**: 실제 열린 커넥션에서 `PRAGMA journal_mode` 질의 결과가 `wal`임을 관찰(구성 문자열이 아니라 엔진 응답).
  3. **span 원자성**: 진행 중인 span 중간에 프로세스를 강제 종료하는 시나리오에서 레코드 상태가 부분 반영 없이 회복됨을 관찰.
- **위험**: 3번은 프로세스 킬 타이밍이 필요해 테스트가 불안정할 수 있음 → 서브프로세스 기반으로 하거나 1·2번만 착수하고 3번은 별도 분리. 고정 sleep 금지(테스트 규율): busy 해제는 실제 커넥션 close 이벤트를 기다린다.
- **수용 기준**: (a) 새 테스트가 현 HEAD에서 green, (b) 구성이 실제로 깨졌을 때(예: WAL 아닌 모드로 open) red임을 수동 확인, (c) B2/B3 baseline green.

### T3. (보류 — 현 구조로 실행 불가) 데몬 측 런타임 통계 수집

- **근거와 한계**: go.dev/doc/diagnostics가 `runtime.ReadMemStats`, `runtime.NumGoroutine`을 프로세스 건강 모니터링 표준으로 명시하지만, `mcp service status`는 status CLI가 **자기 자신의** 프로세스 통계만 읽을 수 있다. 짧은 CLI 프로세스의 메모리 값을 노출하는 것은 데몬 메모리와 무관한 오보에 불과하다. 데몬(서비스) 프로세스 안에서 수집한 값이어야 유효한데, 그러려면 서비스 프로세스에 값 노출 경로(예: T1의 endpoint 또는 상태 파일)가 먼저 필요하다.
- **판정**: **지금은 actionable하지 않다.** T1을 먼저 수용해 서비스 프로세스 관측 경로가 생긴 뒤에만 재검토한다. 단독으로는 실행하지 않는다.

## 5. 실행 불가/보류로 표시한 것 (speculative — 권장하지 않음)

- sqlstore 테스트 시간(16.2s) 단축: 원인 규명에 프로파일이 필요하므로 T1 확보 후 재판단.
- synchronous=OFF 등 durability trade-off 변경: state 무결성이 하네스 불변식이므로 공식 문서의 "NORMAL이면 checkpoint만 fsync" 구성이 이미 최적 부근. 근거 없는 변경.
- MCP transport 교체(예: custom Unix socket binding): 현 Streamable HTTP가 스펙 바인딩과 일치(§2)하고 스펙도 custom transport보다 표준 바인딩을 전제. 필요성이 관찰되지 않음.

## 6. 검증 한계

- 외부 CLI 동작(Claude/Codex 호스트의 MCP 렌더링)은 이번 범위 밖.
- 서비스 장기 실행 메모리 실측은 서비스 프로세스 관측 경로(T1)가 선행되어야 가능 — status CLI 프로세스 자체의 메모리 값은 데몬 메모리가 아니므로 근거로 사용하지 않는다. 본 문서에서는 관측 불가를 명시하고 가설로만 기록했다.
- §3 B2의 sqlstore 테스트 실행(16.2s)은 보고서 작성 중 확장 실행으로, 공식 소스 조사 범위를 벗어난 참고값이다. 이 값에 기반한 개선 과제는 두지 않았다.
- modernc v1.53.0의 번들 SQLite 버전은 소스 바이너리가 아닌 CHANGELOG 텍스트로 확인했다 (`CHANGELOG.md:27`, 읽은 원문 링크: https://sqlite.org/releaselog/3_53_2.html).
