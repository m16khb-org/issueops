# 추가 보고서 검증 digest

조회일: 2026-10-02. 대상: [sources](claudedev-sources.md), [content](claudedev-content.md), [spec](streamable-spec.md), [boundary](streamable-boundary.md), 보충 [개정판 교정](mcp-revision-correction.md).

**결론:** Claude.dev는 Anthropic의 기술 블로그다. 최신 MCP HTTP 계약과 issueops의 현재 SDK·native actor 계약은 서로 다르다. transport 변경만으로 인증, durable job, cache 절감이나 성능 개선을 얻는다는 결론은 채택하지 않는다. 아래 원문과 코드가 근거다.

## 1. 명시적 교정

- **최신 정식판은 2026-07-28이다.** [발표](https://blog.modelcontextprotocol.io/posts/2026-07-28/)와 [공식 색인](https://modelcontextprotocol.io/llms.txt)을 독립 조회했다. `/draft` 대신 버전 고정 [HTTP binding][T26]을 판단 기준으로 삼는다.
- **disconnect 규칙은 버전별로 반대다.** [2025-11-25][T25]는 단절을 취소로 해석하지 말도록 SHOULD NOT 규정한다. 2026-07-28은 요청의 SSE 응답 stream 종료를 취소로 처리하도록 MUST 요구하며, 작업 중단은 SHOULD, 후속 메시지 금지는 MUST다. 이를 모든 transport의 모든 연결 종료로 확대하지 않는다. 최신 stdio는 요청별 `notifications/cancelled`를 사용한다([취소 규칙][C26]).
- **content의 마지막 HTTP 단락은 역사적 설명이다.** 인용된 [2025-06-18][T18]에도 disconnect 비취소, GET, 선택적 session ID·replay가 있다. 읽은 보고서에는 이미 역사적 계약이라는 교정 표지가 추가돼 있었다. 이 단락을 현재 계약으로 재인용하면 안 된다.
- **Claude.dev를 원격 MCP 소비자로 부른 표현은 근거가 없다.** [Terms][TERMS]는 Anthropic 운영 사이트라고 명시한다. 읽은 boundary에는 이미 “별도의 원격 소비자”로 교정돼 있었다. 이것도 특정 소비자의 존재나 호환성을 입증하지는 않는다.
- **상대 경로 오류 의심은 철회한다.** `claudedev-sources.md`의 두 `../../../` 링크를 보고서 디렉터리 기준으로 해석했으며, 각각 저장소의 `internal/adapter/inbound/catalog/mcp/catalog.go`와 `cmd/issueops/mcpcli/mcp_stream_test.go`에 도달하고 파일이 존재했다. 다만 catalog의 `#L18-L44`는 `DispatchMap` 정의를 포함하지 않는다. 그 정의는 [L60-L68](../../../internal/adapter/inbound/catalog/mcp/catalog.go#L60-L68)이다. stream test의 `#L30-L36`도 resource assertion 전체를 포함하지 않으므로 [도구·resource 테스트](../../../cmd/issueops/mcpcli/mcp_stream_test.go#L34-L44)로 범위를 보완한다. 경로 깊이가 아니라 인용 범위의 교정이다.
- **선택적 discovery의 주체를 구분한다.** 보고서들의 “선택적 `server/discover`”는 client 호출이 선택적이라는 뜻이다. 최신 [versioning][V26]은 서버의 구현을 MUST 요구한다.

## 2. 유지하는 transport 결론

[T26]의 Sending/Receiving Messages와 Request Metadata를 직접 대조했다.

| 항목 | 2026-07-28의 계약 |
|---|---|
| 요청 | 단일 endpoint에 POST한다. `Accept`에는 JSON과 SSE를 모두 포함한다. |
| 응답 | 일반 요청은 JSON 또는 요청별 SSE이며 client는 둘 다 지원해야 한다. |
| 수명 | initialize/initialized, transport session ID, 독립 GET stream, `Last-Event-ID` 재개를 제거했다. 변경 알림은 `subscriptions/listen` POST의 SSE 응답이다. |
| metadata | 요청별 version·clientInfo·capabilities를 전달한다. `MCP-Protocol-Version`과 body가 일치해야 한다. `Mcp-Method`는 모든 request, `Mcp-Name`은 tools/call·resources/read·prompts/get에 필요하다. body 처리 서버는 불일치를 400/`HeaderMismatch`로 거부한다. |
| 보안 | Origin 검증은 MUST이며 존재하는 Origin이 잘못되면 403이다. loopback bind와 적절한 인증은 SHOULD다. |

**MCP session ID는 authenticated identity가 아니다.** 역사적 [보안 규칙][SEC25]은 sessions를 인증에 사용하지 말도록 MUST NOT 명시한다. 최신 clientInfo도 client가 제공하는 metadata다. 설치 SDK는 session ID와 별도로 token의 UserID를 비교한다(`SDK/mcp/streamable.go:295-315`). session ID, client 이름, native session ID, lease generation을 같은 권위로 취급할 수 없다.

**Streamable HTTP 자체는 durable job 계약이 아니다.** 최신 [Tasks 설명][TASKS]은 별도 `io.modelcontextprotocol/tasks` 확장, 양쪽 opt-in, 응답 전에 durable task 생성, taskId·TTL·결과 조회를 설명한다. stream 단절 취소와 이미 생성된 task의 `tasks/cancel`은 별개의 수명이다. 취소가 이미 발생한 외부 효과를 rollback한다는 보장도 없다([C26]의 완료·취소 경쟁 조건). durable 저장과 중복 효과 방지는 서버 애플리케이션의 별도 검증 대상이다.

## 3. issueops native actor 경계

직접 읽은 코드의 경로는 다음과 같다.

1. [mcp_transport.go:9-23](../../../cmd/issueops/mcpcli/mcp_transport.go#L9-L23)는 host 자식 프로세스 안에서 요청을 처리한다. [mcp_sdk_server.go:255-271](../../../cmd/issueops/mcpcli/mcp_sdk_server.go#L255-L271)는 `IOTransport`를 사용하고 stdio EOF 뒤 이미 받은 요청을 drain한다. 이 정책을 최신 HTTP 취소 정책으로 복사할 수 없다.
2. [execution 변환:54-75](../../../cmd/issueops/mcpcli/mcp_tool_issueops_execution.go#L54-L75)는 caller가 보내는 receipt와 별도로 **서버의 `os.Getpid()`**에서 ancestry를 관측한다. [process observer:77-166](../../../internal/adapter/issueops/execution_process.go#L77-L166)는 단일 `ps` snapshot의 PID·parent·시작 시각·실행 파일로 계보를 만든다.
3. [domain:23-51](../../../internal/domain/issueops/native_actor.go#L23-L51)는 receipt가 관측 계보에 정확히 포함돼야 통과시킨다. [application:10-19](../../../internal/application/issueopscycle/native_actor.go#L10-L19)는 live process를 재검사한다. [기존 테스트:77-99](../../../internal/adapter/issueops/execution_process_ancestry_test.go#L77-L99)는 계보 누락과 시작 시각 불일치 거부를 검사한다. 이번에는 실행하지 않았다.
4. [holder 검사:11-45](../../../internal/domain/issueopsauthorization/authorization.go#L11-L45)는 active holder의 host/session/agent와 관측 receipt를 확인한다. [mutation authority:33-41](../../../internal/application/issueopscycle/mutation_authority.go#L33-L41)는 canonical workspace를 확인한다. [release:57-63](../../../internal/domain/issueopslease/release.go#L57-L63)는 active generation·holder·cwd를 요구한다. 다만 execution 없는 record는 holder 검사에서 통과하므로 이를 전체 catalog의 보편적 보호로 일반화하면 안 된다.

**추론:** 독립 HTTP 서버의 ancestry는 원격 요청자의 계보가 아니다. 현재 검사를 그대로 옮기거나 payload PID를 신뢰하는 것으로 요청자 인증을 해결할 수 없다. 위 observer와 validator, [기존 daemon ADR](../../adr/2026-09-23-issueops-mcp-serves-in-process-the-shared-daemon-leaves-the.md)이 근거이며 신규 HTTP 실험 결과는 아니다. boundary의 stdio 유지 권고는 이 계약을 보존하는 판단으로 채택한다.

[go.mod:33](../../../go.mod#L33)는 SDK v1.6.1을 고정한다. 설치 `SDK/mcp/shared.go:32-64`의 지원 목록은 2025-11-25까지다. `RequestExtra`의 TokenInfo/Header(`:476-498`)와 issueops [handler:76-94](../../../cmd/issueops/mcpcli/mcp_sdk_server.go#L76-L94)를 대조하면 이 handler는 Extra를 actor 권위에 연결하지 않는다. SDK의 HTTP 기능이나 Stateless 옵션을 최신판 호환성과 동일시하지 않는다.

`SDK/`는 직접 읽은 `$HOME/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.6.1/`이다.

## 4. Claude.dev에서 유지하는 적용 후보

[Terms][TERMS]와 비용 글의 HTML JSON-LD는 운영·publisher를 Anthropic으로, 저자를 Addy Osmani로 명시한다. [workflow][WF]는 Thariq Shihipar·Sid Bidasaria, [skills][SK]와 [cache][CA]는 Thariq Shihipar의 Claude Code 팀 소속을 본문에서 확인한다. 공급자 글 여러 개를 독립 성능 재현 여러 건으로 세지 않는다.

- [skills][SK]의 gotcha·폴더·점진적 공개와 [context][CTX]의 중복 지침 축소는 제한된 실험 후보다. context 글의 “80% 이상 삭제, 평가 손실 없음”은 Anthropic 내부 주장이지 issueops 규칙 삭제의 검증 결과가 아니다.
- [cache][CA]는 안정적인 prefix·도구 순서·deferred schema를 권한다. issueops의 [catalog:13-44](../../../internal/adapter/inbound/catalog/mcp/catalog.go#L13-L44)는 이미 고정 순서를 정의하지만 host의 cache hit를 보장하지 않는다. schema 공개 방식과 HTTP transport는 별도 실험이다.
- [workflow][WF]는 추가 token과 coordination 비용을 경고한다. [비용 글][COST]도 예시를 illustrative로 표시하고 cache writes를 제외한다. 글의 $2.40 대 $3.50 예시는 약 31% 차이지만 issueops의 절감률은 아니다. 완성한 작업의 성공률·retry·turns·전체 token 비용을 함께 측정하는 원칙만 채택한다.

## 5. EXPAND: 중복 제거한 미해결 단서

아래는 수행한 실험이 아니라 네 보고서의 후속 단서를 합친 목록이다.

1. **REVISION:** 정확한 host·SDK 버전별로 handshake era, metadata/header 검증, subscriptions, SSE 종료를 검증한다. 범용 HTTP 설정 지원만으로 최신판 호환을 판정하지 않는다. 근거: [T26], [V26], 설치 SDK 지원 목록.
2. **AUTHORITY/READONLY:** 원격 조회 필요성을 먼저 확인하고 action별 권한·workspace·민감정보 경계를 분류한다. mutation 후보는 서로 다른 native caller, receipt 재사용, stale generation, 교차 workspace 거부를 검증해야 한다. 근거: 위 holder·observer 경로, [boundary](streamable-boundary.md).
3. **CANCEL/DURABILITY/PERF:** 외부 효과 전후 단절, 후속 메시지 중지, 결과 복구, 중복 효과, generation 보존을 분리 검사한다. 같은 payload·동시성의 stdio/HTTP 지연·RSS·프로세스 수 비교는 그 뒤에 한다. 근거: [T26], [C26], [TASKS].
4. **CONTEXT/CACHE/COST:** host별 schema 노출·discovery latency·prefix 안정성·cache read/write를 측정하고, 제한된 지침 축소 전후 성공·누락·권한 위반·완료 비용을 비교한다. 근거: [CA], [CTX], [COST], catalog.
5. **WORKFLOW/FEATURES:** bounded workflow를 기존 경로와 비교하고 실패 항목 보존·replay·token budget의 실제 강제 여부·lease 호환성을 확인한다. mods·effort 글은 [sources의 미해결 lead](claudedev-sources.md#expand-leads)로 남긴다. 근거: [WF], [content의 EXPAND](claudedev-content.md#streamable-mcp와-expand).

## 검증 범위

Source fan-out: 신·구 규범, Claude.dev 원문·HTML metadata, 로컬 코드·테스트·설치 SDK를 대조했다. 모든 외부 링크는 2026-10-02에 조회했다. Claim verification: MCP 규범과 공급자 기술 주장은 단일 권위 출처이며, 로컬 적용은 코드와 교차 확인했다. Access boundary: 차단된 공개 출처는 없었다. host 호환성·성능·HTTP 실행 실험은 하지 않았다. 단일 digest 작성 범위이므로 build/test/self-verify·설치·설정·production 변경은 하지 않았다.

수동 대조에서 모든 유지 결론의 근거와 정정, EXPAND 중복 제거를 확인했다. 로컬 링크 대상은 모두 존재하며 trailing whitespace·충돌 표지는 없었다. `git diff --check -- .issueops/research/agent-updates-2026-10-02/digest-additions.md`는 exit 0이었다. 미추적 파일 누락을 보완한 `git diff --no-index --check /dev/null <digest 경로>`는 공백 오류 출력 없이 exit 1이었다. 이는 신규 파일과 빈 파일의 차이 종료 코드이며 내용의 공백 검사는 별도로 통과했다. 최종 단어 수는 Unicode word regex와 공백 기준 모두 1,800 이하로 검사한다.

[T26]: https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http
[T25]: https://modelcontextprotocol.io/specification/2025-11-25/basic/transports
[T18]: https://modelcontextprotocol.io/specification/2025-06-18/basic/transports
[V26]: https://modelcontextprotocol.io/specification/2026-07-28/basic/versioning
[C26]: https://modelcontextprotocol.io/specification/2026-07-28/basic/patterns/cancellation
[SEC25]: https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices
[TASKS]: https://modelcontextprotocol.io/extensions/tasks/overview
[TERMS]: https://claude.dev/terms/
[WF]: https://claude.dev/blog/a-harness-for-every-task-dynamic-workflows-in-claude-code/
[SK]: https://claude.dev/blog/lessons-from-building-claude-code-how-we-use-skills/
[CA]: https://claude.dev/blog/lessons-from-building-claude-code-prompt-caching-is-everything/
[COST]: https://claude.dev/blog/what-a-task-costs-on-opus-5-5/
[CTX]: https://claude.dev/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models/
