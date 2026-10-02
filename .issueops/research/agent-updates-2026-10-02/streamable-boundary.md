# Streamable HTTP와 issueops의 권한 경계

조사일: 2026-10-02. 결론: **native mutation은 in-process stdio를 유지한다. HTTP는 인증된 읽기 전용 원격 표면에 한해 선택적으로 검토하며, 전면 이전은 권고하지 않는다.** 프로토콜 개정과 transport 선택은 별개의 결정이다. 아래는 구현 계획이 아니라 현재 계약의 적용 범위와 판단을 뒤집을 실험 조건이다.

## 근거와 조사 범위

Source fan-out: 저장소 composition/authorization, 설치된 SDK 구현, 공식 신·구 규격을 직접 대조했다. 검색 snippet은 사용하지 않았다.

Source index: 다음 공식 원문을 모두 2026-10-02에 직접 조회했다.

- [정식 릴리스 발표, 2026-07-28](https://blog.modelcontextprotocol.io/posts/2026-07-28/).
- [최신 Streamable HTTP binding, 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http).
- [이전 transport 규격, 2025-11-25](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports).
- SDK 근거는 설치 경로 `$HOME/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/`에서 읽었다. 이하 `SDK/`는 이 경로다. 대응 소스 URL은 [v1.6.1](https://github.com/modelcontextprotocol/go-sdk/tree/v1.6.1/mcp)이다. URL 원격 내용 대신 설치 소스를 검증했다.

Claim verification: 규격 의미는 공식 규격 단일 권위, 로컬 동작은 코드·기존 테스트·ADR 교차 확인이다. 발표와 규격은 독립된 두 출처로 세지 않는다. HTTP 배포 결과와 성능 향상은 측정하지 않았으며 주장하지 않는다.

Access boundary: 공개 원문 접근 차단은 없었다. secrets/auth 파일과 세션 전사는 읽지 않았고 의존성·설정·설치·런타임 state를 변경하지 않았다.

## 확인된 현재 경계

`cmd/issueops/mcpcli/mcp_transport.go:9-23`은 host가 띄운 자식 프로세스에서 요청을 처리한다. `mcp_sdk_server.go:22-32`는 SDK 서버에 전체 tool/resource를 등록하고, `:255-270`은 `IOTransport`로 실행한다. 분리된 stdio는 stdin EOF 뒤에도 이미 읽은 요청을 drain한다. HTTP listener로 바꾸는 일은 이 composition의 단순한 I/O 교체로 끝나지 않는다.

`cmd/issueops/mcpcli/mcp_tool_issueops_execution.go:54-75`는 **서버의 `os.Getpid()`**에서 ancestry를 관측한다. payload의 host/session/agent, PID/start/executable, cwd, generation과 달리 ancestry는 호출자가 제공하지 않는다. 관측 실패는 빈 ancestry로 이어져 mutation이 fail-closed된다.

정확한 계보는 `internal/adapter/issueops/execution_process.go:77-166`의 단일 `ps -ww -axo pid=,ppid=,lstart=,comm=` snapshot에서 현재 PID부터 부모를 따라 만든다. PID·시작 시각·실행 파일 tuple로 재사용을 구분하고 순환·누락·128개 초과를 거부한다. `internal/domain/issueops/native_actor.go:23-43`은 입력 receipt가 관측 계보에 정확히 포함되는지 확인하며, `internal/application/issueopscycle/native_actor.go:10-19`는 살아 있는 프로세스를 재검사한다.

추가로 `internal/domain/issueopsauthorization/authorization.go:11-45`는 active holder의 host/session/agent와 receipt를 확인한다. workspace 검사는 `internal/application/issueopscycle/mutation_authority.go:33-41`에 있다. generation 검증의 구체적 사례인 release는 `internal/domain/issueopslease/release.go:57-63`에서 active generation·holder·canonical cwd를 모두 요구한다. `internal/application/issueopslease/release.go:36-46,71-111`이 실제 관측과 update validation을 연결한다. canonical 경로 비교는 `internal/adapter/outbound/issueopslease/filesystem.go:10-25`의 절대경로·가능한 symlink 해석·정규화 비교다.

이는 전역적으로 모든 tool이 동일하게 보호된다는 뜻은 아니다. execution이 없는 record는 위 holder 검사에서 통과한다. 원격 노출 전에 **tool과 action별 권한 분류**가 필요하다.

## 독립 HTTP 서버가 바꾸는 것

공유 HTTP 서버의 부모는 각 요청을 보낸 native 세션이 아니다. 현재 observer를 그대로 쓰면 원격 caller는 서버 계보에 없어서 거부된다. 반대로 서버를 띄운 세션의 receipt가 통과하더라도 그것은 **HTTP 요청자의 증명**이 아니다. 요청마다 PID를 받거나 `_meta.clientInfo`를 actor로 채택해 해결하면 self-report를 권위로 승격한다. 같은 OS 사용자에 대한 인증도 특정 agent/session의 lease 소유 증명과 같지 않다.

이는 새로운 추측만이 아니다. 지정 [ADR](../../adr/2026-09-23-issueops-mcp-serves-in-process-the-shared-daemon-leaves-the.md):12-21은 daemon PPID=1 때문에 actor 증명이 실패하거나 daemon을 시작한 세션만 통과한 사례와, peer PID 전달안을 거절한 이유를 기록한다. 다만 이 기록은 과거 daemon 증거이며 신규 HTTP 실험 결과는 아니다.

서버 수명, HTTP 요청 수명, native process 수명, SQLite lease generation은 서로 다르다. 재접속·서버 재시작·취소가 lease를 자동 해제하거나 새 generation을 발급해서는 안 된다. HTTP의 cwd 문자열은 서버 파일시스템에서 해석되므로 원격 workspace 이름을 로컬 경로로 그대로 신뢰할 수도 없다.

## 최신 규격과 설치 SDK를 구분

2026-07-28은 **정식**이다. initialize/initialized와 transport session ID가 사라졌고 요청별 metadata 및 선택적 `server/discover`를 사용한다. 최신 HTTP는 POST 응답에 JSON 또는 요청별 SSE를 사용한다. 독립 GET stream·`Last-Event-ID` replay는 없으며 변경 알림은 `subscriptions/listen` POST의 응답 stream으로 전달한다.

결정적인 수명 차이: 최신 binding은 SSE 응답 stream 종료를 해당 요청의 취소로 처리하도록 **MUST** 요구한다. 2025-11-25는 disconnect를 취소로 해석하지 말라고 **SHOULD NOT** 규정했다. stdio EOF drain 정책을 최신 HTTP에 복사하면 안 된다. 취소는 이미 발생한 외부 효과를 되돌린다는 보장도 아니므로 완료 결과 조회·중복 실행 방지 계약이 별도로 필요하다.

`MCP-Protocol-Version`은 body metadata와 일치해야 한다. `Mcp-Method`는 모든 request, `Mcp-Name`은 tools/call·resources/read·prompts/get에 필수다. body와 불일치하면 400 및 `HeaderMismatch`로 거부한다. 이 검증은 gateway와 backend의 해석을 맞추지만 native actor를 인증하지 않는다.

반면 `go.mod:33`은 v1.6.1을 고정한다. `SDK/mcp/shared.go:32-64`의 최신·지원 목록은 2025-11-25까지이며, 선언된 2026-06-30 상수도 지원 목록에는 없다. `SDK/mcp/streamable.go:126-194`에는 HTTP handler, Stateless, JSONResponse, EventStore replay, SessionTimeout이 존재한다. **Stateless 옵션은 최신 규격 지원 스위치가 아니다.** `:295-346`은 여전히 session ID·DELETE·GET을 처리하며, `SDK/mcp/streamable_client.go:41-71`도 initialize/GET/DELETE 수명을 설명한다.

SDK에는 인증 연결점도 있다. `SDK/mcp/shared.go:476-498`의 RequestExtra는 TokenInfo와 Header를 제공하고 `streamable.go:309-315`는 기존 session의 UserID를 비교한다. 하지만 issueops의 `mcp_sdk_server.go:76-94`는 현재 이 Extra를 actor authorization으로 연결하지 않는다. SDK HTTP 기능 존재와 안전한 issueops 원격 mutation은 별개다. 기본 Host 보호와 Origin 보호도 다르며, `streamable.go:164-186,255-269`상 Origin 보호는 별도 설정/미들웨어가 필요하다.

## 적용 판단과 반대 근거

stdio 유지는 기존 native 실행 계약에 가장 잘 맞는다. 선택적 HTTP는 별도의 원격 소비자에게 **가시성 요구**가 있을 때 검토할 후보이지, 특정 제품·설치 버전의 호환성이 검증됐다는 뜻은 아니다. Claude.dev는 이 조사에서 기술 글 사이트로 확인했으며 MCP 소비자로 확인한 것이 아니다. 명시적으로 선별한 read-only DTO, workspace별 조회 권한, 민감정보 제거가 선행되어야 한다. tool 이름이나 read-only annotation만 믿지 말고 action과 실제 부수효과를 확인해야 한다. 기존 전체 catalog를 그대로 공개하는 방식은 제외한다.

반대 근거도 있다. HTTP는 원격 접근과 gateway 관측에 적합하고, 최신 stateless core는 transport session affinity 부담을 없앤다. 기존 SDK도 HTTP 구성요소를 제공하므로 transport 구현 자체가 막힌 것은 아니다. ADR:14에는 daemon의 p99가 더 낮았던 과거 spike도 있다. 그러나 이것은 HTTP benchmark가 아니며, 프로세스 수·지연·메모리 개선의 증거로 전용할 수 없다.

mutation까지 넓히려면 authenticated principal과 native actor의 위임 관계, 서버 측 workspace binding, generation별 권한 범위·만료·철회, 요청별 재검증, commit 직전 fencing, 취소·재시도·결과 조회의 계약이 필요하다. 로컬 attesting bridge를 선택하면 그것이 새로운 신뢰 경계임을 명시해야 한다. 원격 service principal을 허용한다면 기존 native-only 계약의 보존이 아니라 명시적 확장이다.

## EXPAND: 판단을 바꿀 수 있는 실험

- **EXPAND-ACTOR:** 격리 fixture에서 서로 다른 native 부모의 두 caller와 독립 서버를 비교한다. 정상 owner 성공, 다른 caller의 receipt 재사용·PID 재사용·stale generation·다른 workspace 모두 거부가 통과 조건이다. 기존 `execution_process_ancestry_test.go:77-99`는 ancestry 누락·시작 시각 불일치 거부를 이미 검사하지만 HTTP identity는 검사하지 않는다.
- **EXPAND-REVISION:** SDK/host 버전별로 2025 initialize와 2026 무-handshake request, header/body 불일치, subscriptions, SSE 종료를 구분한다. 최신 의미를 만족하지 못한 client는 호환으로 기록하지 않는다. 발표의 Go 지원 문구는 현재 pin의 호환성 증거가 아니다.
- **EXPAND-CANCEL:** SSE 단절을 외부 효과 전후에 주입한다. 취소 전달·후속 메시지 중지·중복 효과 없음·generation 불변·결과 복구를 각각 관측한다.
- **EXPAND-READONLY-PERF:** synthetic workspace에서 허용 목록 밖 action과 교차 workspace 조회가 거부되는지 검사한다. 같은 payload/동시성으로 stdio와 HTTP의 cold/warm p50·p95·p99, RSS, 프로세스 수, 취소율을 측정한다. 수치 없이 최적화로 채택하지 않는다.

## 검증 기록

코드와 인용한 설치 소스를 직접 읽었다. 경로 탐색 중 `issueopsauthorization/holder.go` 및 `internal/application/issueopsauthorization` 조회가 실패했으며 실제 `authorization.go`와 `issueopscycle/mutation_authority.go`로 바로잡았다. 차단된 공개 출처는 없다. 새 HTTP 서버·host 호환성 실험·성능 측정은 실행하지 않았다. 분석 전용 범위이므로 build/test/self-verify와 runtime state 쓰기는 수행하지 않았고, 문서 수동 대조와 지정 `git diff --check`만 완료 검증 대상으로 삼았다.

`git diff --check -- .issueops/research/agent-updates-2026-10-02/streamable-boundary.md`는 exit 0이었다. 신규 미추적 파일이 diff에서 빠질 수 있으므로 본문도 별도로 검사했고 trailing whitespace는 없었다. 실패한 검증 명령은 없다.
