# CLI, MCP, and output contracts

> Family index: [`../CONVENTIONS.md`](../CONVENTIONS.md). This module owns the
> CLI and MCP tool schemas, the `--json`/exit-code contract, MCP response and
> single-object output shape, the api-doc gate, prompt structure, the
> project-docs bootstrap surface, and commit-message format. Go/package/port/
> adapter rules live in [`go-and-packages.md`](go-and-packages.md); runtime
> state, policy, guard, hook, and lifecycle rules live in
> [`state-policy-and-hooks.md`](state-policy-and-hooks.md).

## 3. CLI 컨벤션

- subcommand는 동사 중심으로 짧게 둔다: `inspect`, `state`, `run`, `mcp`, `worker`.
- 사람이 읽는 기본 출력과 agent가 파싱하는 `--json` 출력을 구분한다.
- `--json` 출력 field는 snake_case를 사용한다.
- 실패 시 stderr에는 짧은 설명, JSON 모드에는 machine-readable error code를 포함한다.
- `--json` 실패는 stdout의 `{"ok":false,"error":...}` 한 문서로만 답한다. 그 오류를 stderr에 다시 쓰지 않는다(`2>&1 | json.load`가 한 문서를 받아야 한다). 오류 envelope을 출력한 명령은 `jsonout.PrintError`나 `jsonout.Reported`로 오류를 표시하고, root command는 표시된 오류를 반복하지 않는다(`cmd/issueops/rootcmd/root_command.go`). 표시되지 않은 오류는 `--json`(passthrough `--` 앞)이 있으면 root가 `{"ok":false,"error":"<command>: <message>"}`를 stdout에 낸다(`rootcmd.JSONRequested`). 결과 DTO를 이미 출력한 명령은 오류를 표시해야 한다. 실패 결과는 `jsonout.PrintResultError`(DTO에 `error`를 합친다)로, 결과 자체가 판정인 경우(`gates` 미충족, policy 거부, `verify-work`, `contract conformance` 판정)는 `jsonout.Reported`로 돌려준다. 표시를 빠뜨려도 root는 두 번째 문서를 쓰지 않는다. `jsonout.PrintTo`가 os.Stdout에 문서를 쓴 실행이면(`WroteStdoutDocument`) 오류를 예전처럼 stderr 한 줄로 낸다. 그래서 CLI JSON은 직접 encoder가 아니라 `jsonout`으로 출력한다.
- `--help`는 usage에 나열된 모든 명령 경로에서 exit 0과 stdout 출력으로 답한다(`TestEveryListedCommandPathAnswersHelpOnStdout`).
- 명령 플래그는 `fs.Parse` 대신 `cliflag.Parse(fs, args)`로 파싱한다. 잘못된 플래그는 Go의 플래그 목록 덤프 없이 오류 한 줄(또는 `--json`이면 JSON 문서 하나)로 끝나고, `--help`는 플래그 목록을 stdout에 낸다(`cmd/issueops/cliflag`).
- 위치 인자를 받지 않는 명령은 `cliflag.ParseNoArgs`를 쓴다. Go 파서는 첫 위치 인자에서 멈추므로, 쓰지 않으면 `next io-1 --json`이 ID와 `--json`을 조용히 버리고 다른 사이클을 고른다. 이제는 `unexpected positional argument` 오류다.
- 명령 그룹이 모르는 subcommand를 받으면 usage 블록을 stderr에 찍지 않고 `cliflag.UnknownSubcommand`로 유효한 subcommand 목록을 담은 오류 한 줄을 돌려준다. `--json`이면 stdout의 JSON 오류 문서 하나만 남는다(`TestEveryListedCommandPathAnswersHelpOnStdout`이 모든 그룹을 확인한다). subcommand 없이 그룹만 부르면 usage를 그대로 보여 준다.
- record mutation(`intent record`, `phase`, `decision add`, `link-plan` 등)의 `--json` 성공 응답은 `{ok, schema_version, id, phase, branch, worktree_path, updated_at}` 확인 응답이다(`issueopscontract.MutationAck`). 전체 record는 `issueops status --id ID --json`이 돌려준다. `start`도 전체 record를 낸다.
- `--help`·`-h`·선두 `help`의 usage는 stdout으로 나간다(`issueops X --help | grep`). 알 수 없는 top-level 명령은 전체 usage 대신 한 줄 오류와 가까운 명령 후보를 stderr에 낸다.
- record 계열 명령의 RECORD_ACTOR_FLAGS는 `cmd/issueops/issueopscli/actorflags` 한 곳에서 선언한다. `execution whoami`의 claim 벡터(receipt 플래그 포함)도 받으며, receipt는 라이브 native ancestry의 한 항목과 세 필드가 모두 맞아야 하고 아니면 ancestry를 비워 fail-closed로 거부된다.
- exit code는 의미 있게 유지한다(`cmd/issueops/issueopsapp/root_command_facade.go`, `cmd/issueops/rootcmd/root_command.go`).
  - `0`: 성공
  - `1`: 일반 실패. flag/usage 오류도 현재는 `gates`를 제외하면 `1`이다.
  - `2`: 알 수 없는 subcommand, `gates` usage 오류
  - `3`: policy 거부(`policy`)와 guard 차단(`guard`)
  - workspace/config 전용 exit code는 없다. 새 exit code를 도입하면 이 목록과 `usage.golden.txt`를 함께 갱신한다.
- 명령·플래그를 추가하거나 지우면 `internal/adapter/inbound/catalog/cli/usage.go`의 usage 줄과
  `cmd/issueops/testdata/usage.golden.txt`를 함께 고친다. `TestCLIUsageGolden`이 둘을 대조한다.

---

## 4. MCP 컨벤션

- MCP tool 이름은 snake_case `<capability>_<verb>` 형태다(예: `harness_inspect`, `state_write`, `issueops_execution`). 목록은 `cmd/issueops/testdata/mcp_tools.golden.json`이 고정한다.
- CLI와 MCP는 같은 core request/response DTO를 공유한다.
- tool response에는 불필요한 대용량 파일 내용을 싣지 않는다. 요약, 경로, hash, line range를 우선한다.
- tool 결과의 text content와 resource text는 들여쓰기·HTML escape 없는 compact JSON이다(`cmd/issueops/mcpcli` `compactJSON`). 들여쓴 형태보다 약 20% 작고, `structuredContent`가 있는 도구는 MCP 하위 호환 규칙대로 같은 값을 text로도 싣는다. CLI `--json`은 사람이 `head`·`grep`으로 볼 수 있도록 들여쓴 JSON을 유지한다.
- 문서 색인(`issueops docs --json`, MCP `docs_index`, `issueops://docs`)은 기본으로 규칙 문서(index·module·cautions·adr)만 나열하고, 날짜가 붙은 기록(`.issueops/issues`, `research`, `plans`, `archive`, `verified-execution`)은 `history_omitted` 개수로만 알린다. 전부 필요하면 `docs --all` 또는 `include_history: true`를 쓴다(`internal/domain/docs` `HistoryRecord`). 항목은 `issueops_root` 기준 `rel_path`만 싣고(절대 경로 반복 없음), `headings`는 제목을 뺀 절 제목이다. 이 저장소에서 414 KB가 80 KB로 줄었다. 반대로 `project route-docs`는 "이 문서를 읽어라"라는 응답이라 항목마다 절대 `path`를 유지한다(셸 cwd가 저장소가 아니어도 바로 연다).
- command 실행 tool은 policy 결과와 audit log id를 함께 반환한다.
- 도구 자신의 실패(입력 schema 위반, 업무 규칙 거부, 실행 실패)는 JSON-RPC 오류가 아니라 `isError: true` 결과로 돌려준다(MCP 2025-11-25, SEP-1303). 텍스트는 compact JSON `{"ok":false,"error":"<무엇이 실패했나>: <이유>"}`이고, 입력 위반은 `invalid_tool_arguments`와 경로별 진단을 싣는다. JSON-RPC 오류로 보내면 Claude Code는 모델에게 `Invalid params` 한 줄만 보여 주고 `data`의 이유를 버려서 모델이 호출을 고칠 수 없다(2026-10-24 `claude -p`로 확인). 알 수 없는 도구와 내부 오류(-32603)만 protocol error로 남긴다(`cmd/issueops/mcpcli` `toolExecutionError`).
- schema 변경은 golden test와 문서 업데이트를 동반한다.
- 모든 MCP tool은 `annotations.readOnlyHint`를 명시한다(`TestEveryToolDeclaresReadOnlyHint`). 호스트는 hint가 없는 도구를 쓰기 도구로 다룬다. Claude Code는 `readOnlyHint`가 있어야 도구를 병렬로 실행하고, 연결이 끊긴 HTTP 호출은 read-only·idempotent 도구만 다시 보낸다. 분류는 실제 쓰기 경로를 기준으로 `internal/contract/mcp/output_schemas.go`의 생성자 중 하나를 고른다. 읽기만 하면 `ReadOnlyClosedWorldAnnotations`, 공개 네트워크를 읽으면 `ReadOnlyOpenWorldAnnotations`, harness state나 workspace 파일만 쓰면 `LocalWriteAnnotations(additive, idempotent)`를 쓴다. 호출자가 고른 명령을 실행하거나 lease·git을 움직이는 도구는 `UnclaimedWriteAnnotations`를 쓴다. 옵션(`save_state`, `prune_retention` 등)으로라도 쓸 수 있으면 read-only가 아니다.
- CLI/MCP contract golden은 `cmd/issueops/testdata/`에 둔다. 의도된 schema 변경일 때만 `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`로 갱신한다.
- 실제 JSON response golden은 dynamic field(timestamp, temp path, audit id)를 normalize해서 host/session 차이로 인한 drift를 막는다.
- response golden 범위는 state/policy뿐 아니라 docs/inspect/preflight처럼 agent가 자주 의존하는 읽기 표면을 우선 포함한다.
- CLI와 MCP가 같은 `next_command`를 반환할 때 provenance 같은 composition evidence도 동일하게 결합한다. Port에는 contract 타입 대신 순수 observation receipt만 두고 application에서 contract evidence로 변환한다. 파일·프로세스 관측은 contract/core/cmd package에 넣지 않고 `issueopsapp`이 outbound adapter를 생성해 주입하며, 관측 실패를 빈 값이나 `unavailable`로 치환하지 않는다.

---

## API Documentation Gate

Reusable API documentation checks belong in `issueops api-doc check`, not in a single application repository or framework-specific hook. Keep the core prompt framework-agnostic: it may mention examples such as NestJS Swagger decorators, Go swaggo annotations, OpenAPI specs, Spring/FastAPI annotations, but must instruct reviewers to follow the target project's own convention and staged diff only. Project-specific strictness should be supplied via `--prompt-file` rather than hardcoded into harness core.

## Prompt Structure Convention

Reusable harness prompts follow the strong prompt shape from the user-provided example. Do not copy example-specific domain content into harness prompts; reuse the structure:

- Identity: name the role and expertise the model should inhabit.
- Objective: state the concrete outcome.
- Operating phases: break the work into explicit ordered stages.
- Inputs: name the data the model may use and how missing data is represented.
- Rules: state hard constraints, safety boundaries, and decision rules.
- Output contract: define the exact response format and forbidden wrapper text.
- Verification checklist: require a final self-check against the prompt contract.

New reusable prompts should use `internal/domain/prompt.BuildStructuredPrompt` where possible. JSON packet prompts may use equivalent structured keys instead of Markdown headings, but they still need the same identity/objective/phases/inputs/rules/output/checklist shape. Existing prompt-specific strictness, such as JSON-only output or no Markdown fences, must remain stronger than the generic structure.

## Project Docs Bootstrap 컨벤션

- 적용 대상 repo의 `.issueops/` 문서는 명시적 `issueops project bootstrap` 또는 `$project-bootstrap` 실행 때만 생성한다.
- 기본 설치나 MCP read는 대상 repo에 파일을 쓰지 않는다.
- `AGENTS.md`는 전체 덮어쓰기 금지다. bootstrap은 behavioral top block이 없으면 prepend하고, 이후에는 `ISSUEOPS` marker block만 관리한다.
- 생성 문서에는 추론된 명령과 기술스택의 Evidence/Confidence를 포함한다.
- MCP 라우팅은 문서를 무조건 주입하지 않고 작업별로 읽어야 할 문서 경로와 이유만 제공한다.

## API Documentation Convention

OpenAPI 요구사항의 정규 소유자는 [`OPEN_API_SPEC.md`](../OPEN_API_SPEC.md)다. API documentation gates are framework-agnostic and business-logic-aware. A changed endpoint's docs must reflect not only syntax-level params and DTO schemas but also directly reachable domain errors from service/usecase/error-mapping code. Keep Swagger/OpenAPI output clean: concise summaries, consistent sectioned descriptions, complete params, accurate required/optional fields, and explicit success/error responses.


NextCandle-style cleanliness means sectioned Markdown operation descriptions, complete params, accurate auth/error responses, DTO required/optional correctness, and Swagger outputs split/filtered for the intended audience where the framework supports it.

## Single-object response convention

단일 객체 응답은 불필요한 wrapper object로 한 번 더 감싸지 않는다. 공개 계약이 이미 하나의 resource/detail/view model이라면 controller/handler는 `{ data: object }`, `{ result: object }`, `{ item: object }`처럼 포장하지 말고 object fields를 top-level로 spread해 반환한다.

예외는 pagination/list envelope, metadata가 계약의 일부인 경우, backward compatibility 유지, 표준 error envelope처럼 wrapper 자체가 명시적 API 계약인 경우다. 새 endpoint나 DTO 리팩터링 시 OpenAPI schema도 이 규칙과 일치해야 한다.

## 10. 커밋 메시지 컨벤션

- 커밋 메시지는 [`COMMIT_POLICY.md`](../COMMIT_POLICY.md)를 따른다.
- subject는 Conventional Commit 형식(`<type>(<scope>)!?: <summary>`)을 사용한다.
- body에는 AI agent가 맥락을 회수할 수 있도록 `Lore:` 블록을 둔다.
- atomic commit 하나에는 Lore `Intent`도 하나만 있어야 한다.
