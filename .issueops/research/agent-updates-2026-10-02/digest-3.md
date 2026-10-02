# Shared / IssueOps evidence digest

검증일: 2026-10-02. 대상은 `shared-01`~`shared-10`, `issueops-01`~`issueops-10`과 리드의 직접 증거다. 공개 원문과 인용 코드만 대조했으며 구현안을 제시하지 않는다.

**결론:** 현재 근거가 확정하는 것은 결과 표현의 손실, 관측 데이터의 불완전성, 문서와 현재 코드의 차이다. 성능 개선량이나 새로운 SDK·HTTP·telemetry 계층의 필요성은 확정하지 못한다. 특히 SDK 기능 부재와 JSONL 결함의 미재현이라는 전제는 교정해야 한다.

## 근거 수준과 원문 검증

Source fan-out: MCP 규격·SDK, 두 provider의 캐시 문서, OTel·Agent Skills 규격, 평가·durable 실행 문서, 로컬 구현·이웃 테스트를 대조했다. 영향이 큰 다섯 주장 묶음은 아래처럼 원문을 직접 다시 가져왔다.

1. **MCP revision / Tasks:** MCP `2026-07-28`은 정식 공개됐다[S1]. Tasks는 extension이며 `tasks/get`, `tasks/update`, `tasks/cancel`을 정의한다. 요청에 extension capability가 없으면 task 결과를 반환하면 안 되고, handle 반환 전에 durable 생성이 완료돼야 한다[S2]. SEP-2663은 스스로 historical record라고 명시하므로 현행 버전 원문을 우선했다. Task 상태 알림과 일반 progress는 같지 않다. 현행 Tasks 원문은 task에 `notifications/progress`·`notifications/message`를 지원하지 않는다고 명시한다.
2. **HTTP cancellation:** 새 Streamable HTTP에서 요청의 SSE 응답 stream 종료는 취소다. 독립 GET stream·session ID·`Last-Event-ID` 재개는 제거됐다[S3]. 이는 stdio의 명시적 취소와 구분된다. 현재 IssueOps stdio 호출 경로는 `cmd/issueops/mcpcli/mcp_transport.go:14-26`이며, [리드 관측](mcp-baseline.md)은 `2025-11-25` 협상 한 건이지 전체 지원 버전 목록이 아니다.
3. **구조화 결과 / SDK:** 출력 스키마는 선택 사항이지만 제공하면 conforming structured result가 필요하다. Annotations는 enforcement가 아닌 hint다[S4]. 고정 SDK `v1.6.1`의 `mcp/protocol.go:71-90,1325-1383`에서 `StructuredContent`, `OutputSchema`, `ToolAnnotations`를 직접 확인했다. 최신 SDK는 이 표현들의 필수 전제가 아니다.
4. **Provider cache:** OpenAI 문서는 GPT-5.6 이후 explicit breakpoint, 1,024 visible-token 최소 prefix, read/write 분리 과금을 명시한다[S5]. Anthropic은 일반적인 5분 write 1.25배·1시간 write 2배, workspace별 격리와 provider별 예외를 명시한다[S6]. 어느 문서도 IssueOps가 해당 usage를 받거나 비용을 절감했다는 증거가 아니다.
5. **관측 규격 / Skills:** OTel agent 문서는 Development이며 remote invocation은 CLIENT, 같은 process 내부 invocation은 INTERNAL이다[S7]. Agent Skills는 description 최대 1,024자, optional `compatibility`, body 5,000 tokens 미만 권장, 500줄 미만 권장을 정의한다[S8]. 형식 이식성과 실행·활성화의 동일성은 별개다.

**Primary-only exceptions:** MCP 규격·SEP·blog·SDK release는 같은 생태계의 규범/구현 기록으로 취급했다. Provider·OTel·Skills 문서도 자기 제품/규격에 대한 1차 근거이며 독립 성능 재현이 아니다. 이웃 테스트는 의도된 계약을 보강하지만 이 digest에서 실행하지 않았다. 리드 CLI 재현은 명시적으로 리드 관측으로 귀속한다.

## 교정 사항

- **shared-03 / issueops-01:** 구조화 결과를 검토하려면 SDK 업그레이드가 필요하다는 전제는 기각한다. IssueOps의 shared catalog는 세 필드만 전달하고, 등록·변환 경계는 text만 보존한다(`internal/contract/mcp/catalog_types.go:3-8`; `cmd/issueops/mcpcli/mcp_sdk_server.go:98-152`). 자동 출력 검증은 typed `AddTool`과 schema draft 조건에 달려 있으며 필드 존재만으로 보장되지 않는다. [SDK 직접 증거](mcp-output-verification.md).
- **issueops-05:** JSONL 손실은 더 이상 정적 가설만이 아니다. [리드 대조 재현](trace-jsonl-reproduction.md)은 padding만 100→70,000자로 바꾸면 뒤의 `sentinel_failure`가 누락되고 exit 0·`ok:true`가 유지됨을 기록한다. 두 행의 finding count는 1→0, 세 행은 2→1이다. `internal/adapter/trace/decode.go:26-38`도 기본 Scanner와 누락된 `scanner.Err()` 확인을 직접 보여준다. 성능 병목이 아니라 분석 완전성 결함이다.
- **shared-05 / shared-06:** 2026-07-02 usage-writer ADR을 현재 writer 증거로 쓰면 안 된다. ADR이 지목한 `internal/core/externalllm/usage.go`, `internal/core/external_llm_usage.go`는 없고, 실행 소스 검색에서 `external-llm-usage`는 retention 테스트에만 남는다(`internal/adapter/outbound/state/state_test.go:340-358`). 현재 provider·token accounting은 미해결이다.
- **shared-04:** `invoke_agent`를 일괄 CLIENT로 매핑하는 단정은 부정확하다. 원문에는 같은 process용 INTERNAL span도 있다[S7].
- **shared-08:** source inventory를 직접 확인한 결과 `skills/*/SKILL.md`는 52개다. `.issueops/architecture/host-integration.md:39`의 52와 일치하지만 `.issueops/TECH_STACK.md`의 “34개”와 다르다. 이는 source 수량이며 host별 설치·발견 수량은 아니다.
- **issueops-09:** duration 손실은 contract check에만 있지 않다. `internal/adapter/verification/probe/contractauditworker/` 아래 `validation_contract_check.go:47`, `validation_command_audit.go:42`, `validation_tool_conformance.go:47`, `validation_worker_lifecycle.go:46` 모두 `time.Since(time.Now()).Milliseconds()`로 앞선 실행 시간을 제외한다.

## 로컬 공백 대조 결과

아래는 각 lane의 후보 공백을 코드와 대조한 결과다. 확인된 구조와 미측정 효과를 분리한다.

| Lane | 확인한 경계와 남은 증거 |
|---|---|
| issueops-01 | catalog는 12개 advertised section과 alias section을 합친다(`internal/adapter/inbound/catalog/mcp/catalog.go:18-80`). Omo digest invalidation은 이미 있다(`cmd/issueops/issueopsapp/host_installers.go:67-77`; `internal/adapter/omo/mcp.go:47-65`). 51 tools / 30,888 bytes는 리드 단일 관측이며 매 턴 token 비용은 미해결이다. |
| issueops-02 | root-wide span gate와 별도 lock DB, 데이터 CAS는 다른 경계다(`internal/adapter/outbound/sqlstore/sqlstore.go:203-241,340-411,747-780`). `GetExisting`은 context-free, streaming은 QueryContext다(`:496-579`). wait/hold 관측과 WAL checkpoint 결과는 이미 있다(`internal/adapter/outbound/issueopsrecord/observer.go:16-93`; `sqlstore/maintain.go:16-34`). query·Open·visitor별 시간과 production contention은 미측정이다. |
| issueops-03 | staged build·activation·seal은 이미 존재한다(`scripts/install-native.sh:160-248`). preflight Ready는 executable/version 확인이다(`internal/adapter/hostprobe/runner.go:189-221`); 별도 live episode도 있다(`hostprobe/codex.go:86-115`). generation 표시는 revision+dirty라 binary 고유 digest와 다르다(`internal/adapter/install/native_generation.go:12-72`). Claude config는 `alwaysLoad`를 지정하지 않는다(`internal/adapter/claude/install_mcp.go:16-25`); 현재 eager loading 여부는 미해결이다. |
| issueops-04 | SessionStart menu discovery는 bounded body reads를 수행한다. raw `ReadDir(128)` 뒤 최종 sort는 capped membership을 고정하지 않고, accepted bytes만 합산한다(`internal/adapter/projectdoc/catalog.go:12-122`). skip 이유는 반환하지 않는다. host payload 차이(`internal/adapter/hostprotocol/hook.go:5-19`)와 on-demand whole-file read(`internal/adapter/projectdocs/project_docs_revise_effects.go:11-19`)는 실제지만 startup 지연은 미측정이다. |
| issueops-05 | trace decoder·usage ADR 교정은 위와 같다. review phase duration은 완료 timestamp 차이며 human/idle 시간을 포함할 수 있다(`internal/domain/issueops/review_metrics.go:70-120`). 실행 소스의 W3C context 검색은 일치가 없었으며 host-owned tracing까지 부정하지 않는다. |
| issueops-06 | root 기본 wiring은 비어 있다(`cmd/issueops/issueopsapp/cli_facade.go:16-18`). Git preflight는 성공 경로에서 10/11 subprocess를 유도한다(`internal/adapter/preflight/preflight.go:17-57`). direct Git의 buffer/deadline 경계(`preflight/git.go:16-31`)는 policy runner의 bounded capture·group kill·drain(`internal/adapter/policy/policy_run.go:53-160`)과 다르다. 병목은 미측정이다. |
| issueops-07 | branch/status cache와 next changed-path snapshot은 이미 request/operation 범위로 존재한다(`internal/adapter/issueops/readiness_git.go:16-54`; `cmd/issueops/issueopsapp/issueops_next_wiring.go:35-118`). 같은 root worktree 재관측과 plan path 중복 검사는 남는다(`internal/application/issueopsnext/service.go:140-185,329-336`; `internal/application/issueopscycle/readiness_service.go:147-155`). fetch 뒤 refresh는 의도된 계약이다. |
| issueops-08 | released와 claimable resume는 다르다(`internal/domain/issueopslease/resume.go:63-95`). 기존 live binding 재사용과 invocation marker는 존재한다. 실패 기록은 cancellable context를 재사용하고 오류를 버린다(`internal/application/issueopslease/resume.go:79-132`); context-aware CAS 때문에 진단 기록 실패 가능성은 코드로 확인되지만 운영 재현은 미해결이다. cycle fence는 별도 root다(`internal/adapter/outbound/issueopslease/reseed_fence.go:32-38`). |
| issueops-09 | self-verify는 single pass이며 성공 suite evidence 재사용이 이미 있다(`internal/application/selfverify/loop.go:65-123`; `steps.go:74-95`). 단일 관측의 p95는 그 값 자체다(`internal/domain/selfaugment/step_stats.go:34-59`). duration 결함과 실제 지연은 별개다. |
| issueops-10 | Inspect는 hardcoded Codex/Claude 중심이며 전체 skill body를 읽는다(`internal/adapter/inspect/inspect.go:14-78`). validator는 `compatibility`를 허용하지 않고 description 길이를 검사하지 않는다(`scripts/validate-skill.py:15-16,98-142`). malformed host filter는 의도적으로 fail-open이다(`internal/adapter/installutil/skill_hosts.go:12-43`). native discovery parity는 미해결이다. |

shared-07의 vendor token 절감률, shared-09의 contamination·비용 수치는 이번 digest에서 독립 재검증 결과로 채택하지 않았다. Anthropic infrastructure 원문은 resource 조건에 따른 infra error 5.8%→0.5%, 1x~3x score 차이 p=0.40을 구분한다[S12]. benchmark DTO에는 비용 필드가 없지만 다른 관측 전체의 부재를 뜻하지 않는다(`internal/contract/issueopsbenchmark/benchmark_run_types.go:22-30`).

shared-10의 durable 실행은 vendor별 계약이다: Temporal history replay[S13], LangGraph node 재시작·checkpoint mode[S14], Inngest의 외부 성공 후 receipt 누락 중복 위험[S15]을 IssueOps 성능 결과로 전환하지 않는다. 로컬 retry identity 검증은 이미 있다(`internal/adapter/orca/client_orchestration.go:552-583`).

## EXPAND: 중복 제거한 조사 질문

1. **결과·발견 경계** (shared-01~03,07; issueops-01,03): 실제 host별 협상 revision, catalog 로딩량, native deferral, structured-result 보존은 무엇인가? 현재 SDK 지원과 최신 revision 지원은 따로 확인한다.
2. **분석 완전성·duration** (issueops-05,09): 리드 JSONL 대조에서 누락된 실패와 네 probe의 제외된 시간이 downstream 판단을 어떻게 왜곡하는가?
3. **현재 비용 근거** (shared-05,06,09; issueops-05): 현재 writer 소유자와 host별 read/write/input usage 가용성은 무엇인가? cache hit·TTFT·완료 시간·청구 비용을 구분할 근거가 있는가?
4. **대기·반복 관측** (issueops-02,04,06~09): matched 조건에서 Open/query/visitor, local gate/SQLite, subprocess/path probes, hook discovery, update stage가 각각 차지하는 시간은 얼마인가?
5. **복구 완전성** (shared-10; issueops-08): 외부 invocation 후 cancellation에서 intent·marker·failure receipt·reconciliation은 어떤 상태를 남기는가? 진단 누락과 중복 실행을 구분할 증거가 필요한 상태다.
6. **Skills·metadata·관측 규격** (shared-04,08; issueops-10): 52개 source의 host별 발견·누락·link validity, validator 차이, fail-open filter, OTel revision별 span 종류는 무엇인가? 이식성·성능은 미확정이다.

이는 조사 질문이며 구현·설치·계측 실행 제안이 아니다.

## Source index와 날짜

모든 원문 retrieval은 2026-10-02다. 날짜 없는 live 문서는 공개일/도입일을 추정하지 않았다.

| ID | 직접 확인한 원문 | 날짜·권위 |
|---|---|---|
| S1 | [MCP release](https://blog.modelcontextprotocol.io/posts/2026-07-28/) | 2026-07-28, 공식 |
| S2 | [Tasks versioned text](https://raw.githubusercontent.com/modelcontextprotocol/ext-tasks/main/specification/2026-07-28/tasks.md), [SEP](https://modelcontextprotocol.io/seps/2663-tasks-extension) | revision 2026-07-28; SEP created 2026-04-27, 공식 |
| S3 | [Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) | revision 2026-07-28, 공식 |
| S4 | [Tools](https://modelcontextprotocol.io/specification/2026-07-28/server/tools) | revision 2026-07-28, 공식 |
| S5 | [OpenAI caching](https://developers.openai.com/api/docs/guides/prompt-caching) | live/undated, provider |
| S6 | [Anthropic caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching) | live/undated, provider |
| S7 | [OTel agent spans](https://raw.githubusercontent.com/open-telemetry/semantic-conventions-genai/main/docs/gen-ai/gen-ai-agent-spans.md) | main/undated, Development |
| S8 | [Agent Skills](https://agentskills.io/specification) | live/undated, format owner |
| S9 | [Claude 2.1.287](https://api.github.com/repos/anthropics/claude-code/releases/tags/v2.1.287) | published 2026-10-01T18:00:22Z; `alwaysLoad:false` deferral |
| S10 | [Go SDK 1.8.0](https://api.github.com/repos/modelcontextprotocol/go-sdk/releases/tags/v1.8.0) | published 2026-09-14T08:05:46Z; SetCacheable, 새 revision 추가 없음(2026-07-28은 v1.7.0부터 지원) |
| S11 | [Codex skills](https://developers.openai.com/codex/skills) | live/undated; `.agents/skills`, 초기 목록 budget |
| S12 | [Infrastructure noise](https://www.anthropic.com/engineering/infrastructure-noise) | 본문 공개일 미확정, vendor experiment |
| S13 | [Temporal tasks](https://docs.temporal.io/tasks) | live/undated, vendor |
| S14 | [LangGraph granularity](https://docs.langchain.com/oss/javascript/langgraph/thinking-in-langgraph) | live/undated, vendor |
| S15 | [Inngest idempotency](https://www.inngest.com/docs/durable-execution/guides-and-advanced/idempotency) | live/undated, vendor |

입력 보고서 색인: [shared-01](shared-01.md), [02](shared-02.md), [03](shared-03.md), [04](shared-04.md), [05](shared-05.md), [06](shared-06.md), [07](shared-07.md), [08](shared-08.md), [09](shared-09.md), [10](shared-10.md); [issueops-01](issueops-01.md), [02](issueops-02.md), [03](issueops-03.md), [04](issueops-04.md), [05](issueops-05.md), [06](issueops-06.md), [07](issueops-07.md), [08](issueops-08.md), [09](issueops-09.md), [10](issueops-10.md).

## 실패·미확보 근거 보존

- shared-01의 기존 실행 실패: `ModuleNotFoundError: No module named 'pydantic'`. collect-all retry는 “timed out after entering `go test`; no result for that step or later build/docs gates was captured.” 이 digest는 이를 passing battery로 바꾸지 않는다.
- 기존 탐색 실패는 해당 원문에 남긴다: shared-07의 `EISDIR`, issueops-01/04/06/09의 `ENOENT`, issueops-07의 `tool.glob` unavailable, issueops-08의 `lines is not defined`. issueops-08/10의 no-index check는 exit 1·empty output이며 passing gate가 아니다.
- shared-01의 guessed URLs는 404, shared-02의 구 cancellation URL은 `Page Not Found`로 기록돼 있다. 정규 원문이 확보된 사실과 실패 이력은 구분한다.
- 이번 fetch도 50KB 출력 제한에 걸렸다. OTel 원문을 HTTP 200·146,327 bytes로 다시 읽어 INTERNAL 예외를 확인했고, Tasks는 정규 원문 HTTP 200·34,148 bytes로 확인했다. 누락된 vendor benchmark 세부는 미해결로 유지한다.
- tests/build/host probe/benchmark는 이번 범위에서 실행하지 않았다. 단정은 직접 읽은 코드·원문 또는 명시적으로 귀속한 리드 재현에 한정한다.
