# 553 역할별 서브에이전트 모델 설정 (issueops model, global·local)

## TL;DR

- Summary: `internal/domain/agentmodel`의 host별 상수를 "host × 역할 → (model, effort)" 해석기로 일반화하고, global·local JSON 설정과 `issueops model` CLI, `/io-model` 스킬을 추가한다. execution prepare, `issueops next`, owner 프롬프트, Orca owner 실행 명령이 이 해석기를 쓰게 한다.
- Deliverables: 해석기(domain), 설정 파일 adapter, `issueops model show|set|unset|resolve`, Orca owner 명령의 역할 에이전트 주입, 소비 지점 연결, `skills/io-model`, 관련 스킬·문서·ADR 갱신.
- Effort: Large
- Parallel: YES — 3 waves
- Critical Path: T1 → T2 → T4 → T6 → T8 → F

## 사이클 경계

- lifecycle ID: `io-e66ba158272a`, issue #553, branch `553-agent-model-config`, base `main`.
- 사용자 요청 범위: 이슈 생성부터 진행 전체(`/issueops로 진행`). 종료점은 draft PR 발행과 `execution complete`다. merge와 머지 후 정리는 별도 승인이다.
- 브랜치·worktree 준비 뒤에는 환경별 자동 세션 인계를 따른다. 구현 owner는 `claude-opus-5-5 / high`로 띄운다. 인계는 요청 범위를 넓히지 않는다.

## Context

### Original Request

이슈 #553 본문과 intent record의 raw request를 따른다. 역할별 서브에이전트 모델을 Claude Code와 Codex별로, global과 local 범위로 설정하고, issueops가 그 목적의 작업을 할 때 해당 모델로 실행되게 한다.

### Interview Summary

- local은 커밋하지 않는다. 워크트리에서도 메인 워크트리의 local을 읽고, 없으면 global, 없으면 내장 기본값을 쓴다.
- 역할은 `implement`, `child-implement`, `plan-review`, `diff-review`, `review-escalate`, `research`, `reader-check` 7개다.
- 적용 방식은 설정 파일 하나와 실행 시점 주입이다. 네이티브 에이전트 정의 파일을 미리 만드는 방식은 기각했다.
- research와 reader-check에도 기본 모델을 지정한다.
- Codex는 `gpt-6-astra`(리뷰 high, docs-only medium), `gpt-6.1-sol`, `gpt-6-luna`만 쓴다. terra는 쓰지 않는다.
- Claude `implement`는 `claude-opus-5-5 / high`, `reader-check`는 `claude-haiku-5-5 / medium`이다.
- 카탈로그에 없는 모델은 경고만 하고 저장한다. 잘못된 host·역할·effort는 거부한다.
- omo는 범위 밖이며 기존 기본값을 유지한다.

### Gap Analysis

- **owner binding 봉인 시점.** Orca 모드에서만 owner host·model·effort가 probe를 거쳐 `OrcaBinding`에 봉인된다(`internal/application/issueopspreparation/orca_receipt.go:96-100`). resume은 binding에서 probe를 재구성하고(`internal/domain/issueopspreparation/resume_begin.go:40,59`) `bindingsEqual`(`internal/domain/issueopspreparation/intent_authority.go:129-137`)과 `ValidateOwnerResumeProfile`(`internal/domain/issueops/owner_resume.go:84-89`)이 일치를 요구한다. 따라서 해석기는 `normalizeOwnerDefaults`(`internal/application/issueopspreparation/prepare.go:373-385`) 한 곳에서만 owner 값에 반영한다. resume 경로는 binding만 쓴다.
- **issueops가 owner 세션을 직접 띄우는 경로는 두 개다.** (1) Orca: `ownerAgentCommand`(`internal/adapter/orca/client_decode.go:226-260`)가 terminal 명령을 만든다. (2) cmux: released direct execution에서 `issueops execution handoff-cmux`가 `hostprotocol.BuildInteractiveArgv`로 native host argv를 만든다(`cmd/issueops/issueopsapp/issueops_cmux_wiring.go:42,55`, `internal/adapter/hostprotocol/argv.go:16-59`). `prepareDirect`(`prepare.go:291`) 자체는 세션을 띄우지 않는다. Herdr와 수동 인계는 Go 코드가 없고 스킬 문서(`skills/issueops/references/session-choice.md:358-384`)가 명령을 조립한다. 세 경로 모두 같은 주입 인자 생성 함수 하나를 쓴다. Herdr는 `issueops model resolve --agents`의 `agent_args`를 받아 쓴다.
- **Orca intent 실패 분류.** Orca 호출 adapter는 `*port.OrcaError{Invoked:false}`가 아닌 error를 모두 `InvocationUnknown`으로 분류한다(`internal/adapter/outbound/issueopspreparation/orca.go:85-91`). `MarkInvoking`(`prepare.go:233`)을 지난 뒤 unknown이 되면 authoritative zero에서도 재시도가 거부된다(`prepare.go:223-226`). 따라서 역할 에이전트 해석과 Codex role 파일 쓰기는 `MarkInvoking` 전에 application에서 끝내고, 결과 인자를 Orca 요청에 싣는다.
- **설정 오류의 처리.** `issueops next`는 관측 실패를 경고로 남기고 진행한다(`internal/application/issueopsnext/service.go:44-49,120-125`). 설정 해석 실패도 같은 방식으로 처리한다. 경고에 파일 경로를 넣고 `review.model`과 `review.effort`를 비운다. 내장 기본값으로 조용히 대체하지 않는다. owner 프롬프트는 review 값이 없으면 리뷰를 멈추는 규칙을 이미 갖고 있다(`internal/adapter/issueops/testdata/execution_owner_prompt.txt:163-165`). `execution prepare`와 `issueops model`은 설정 오류면 어떤 상태도 바꾸기 전에 파일 경로가 담긴 에러로 끝낸다.
- **child start는 모델을 고르지 않는다**(`internal/contract/issueops/types.go:272-280`, `internal/application/issueopsdelegation/start.go:35-86`). child의 owner 모델은 그 child의 `execution prepare`에서 정해진다. record의 `Delegation`(`types.go:352`)이 비어 있지 않으면 `child-implement` 역할로 해석한다.
- **3~5라운드 상향은 스킬 산문에만 있다**(`skills/issueops-review/SKILL.md:159-173`). 이것을 `issueops model resolve --round N`으로 옮긴다. 같은 스킬이 인용하는 Fable 근거(`internal/contract/issueopspreparation/prepare.go`)는 stale하므로 ADR 인용으로 바꾼다.
- **설정 형식은 JSON이다.** `.issueops/TECH_STACK.md:74`는 설정·상태를 표준 `encoding/json`으로 직렬화하고 yaml·toml 라이브러리를 쓰지 않는다고 정한다. 사용자가 2026-10-08에 처음 합의한 TOML 대신 JSON을 선택했다. 이슈 본문의 `.toml` 경로는 계약 변경으로 기록하고 갱신한다. Codex에 주입하는 역할 파일만 Codex가 요구하는 TOML이며, 고정 키 몇 개를 렌더하는 순수 함수로 만든다.
- **설정 경로 정책 충돌.** `.issueops/conventions/state-policy-and-hooks.md` §6은 `~/.config/issueops/`를 예약 경로로 두고 "loader 미구현이므로 읽거나 만들지 않는다"고 적는다. 향후 우선순위는 `flag → env → workspace config → user config → default`다. 이 계획은 그 순서 안에서 env 층을 비워 두고 workspace(local) → user(global) → default를 구현한다. 조항 갱신과 ADR이 필요하다.
- **owner 프롬프트 테스트가 값을 고정한다.** `TestOwnerArtifactsRouteModelRoles`(`internal/adapter/issueops/execution_owner_packet_test.go:498-541`)가 host별 reviewer·research 값을 하드코딩한다. 기본값 변경과 함께 갱신한다.

## Work Objectives

### Core Objective

issueops가 목적별 세션과 서브에이전트를 띄울 때 쓰는 model·effort를 사용자가 global·local 설정으로 정하고, 그 값이 실제 실행에 쓰이게 한다.

### Deliverables

1. 해석기와 내장 기본값 표(domain).
2. 설정 파일 경로·읽기·쓰기·exclude 등록 adapter.
3. `issueops model show|set|unset|resolve` CLI.
4. Orca owner 실행 명령의 역할 에이전트 주입.
5. prepare, next, owner policy, owner 프롬프트 연결.
6. `skills/io-model`과 관련 스킬 갱신.
7. 문서와 ADR 갱신.

### Definition of Done

- `go test ./... -count=1`과 `go test -race ./... -count=1`이 통과한다.
- 아래 G1~G9가 모두 EVIDENCE로 채워진다.

### Must Have

- 우선순위: 명시 플래그 > local > global > 내장 기본값. 병합 단위는 필드(model, effort)다.
- local은 메인 워크트리(`git rev-parse --git-common-dir`의 부모)에서 찾는다.
- local 파일을 처음 만들 때 `<common-dir>/info/exclude`에 경로를 추가한다. 이미 있으면 추가하지 않는다.
- global 경로는 `$XDG_CONFIG_HOME/issueops/agent-models.json`이고, 변수가 비면 `~/.config/issueops/agent-models.json`이다.
- 내장 기본값과 상속 규칙:

  | 역할 | claude | codex | omo(기존 유지) |
  |---|---|---|---|
  | implement | claude-opus-5-5 / high | gpt-6.1-sol / high | chatgpt-subscription/gpt-6-sol / max |
  | child-implement | implement 상속 | implement 상속 | implement 상속 |
  | plan-review | claude-opus-5-5 / high | gpt-6-astra / high | chatgpt-subscription/gpt-6-astra / max |
  | diff-review | claude-opus-5-5 / high | gpt-6-astra / high | chatgpt-subscription/gpt-6-astra / max |
  | review-escalate | 원래 리뷰 모델 + effort 한 단계 | 같음 | 같음 |
  | research | claude-sonnet-5-5 / medium | gpt-6-luna / medium | chatgpt-subscription/gpt-6-luna / medium |
  | reader-check | claude-haiku-5-5 / medium | gpt-6-luna / low | 없음(빈 값) |

- docs-only 티어는 리뷰 effort가 **내장 기본값에서 왔을 때만** `medium`으로 낮춘다. 사용자가 effort를 적었으면 그 값을 쓴다.
- `--round N`(N ≥ 3)은 `review-escalate` 설정이 있으면 그 값을, 없으면 원래 리뷰 모델에 effort 한 단계 상향을 쓴다. 상향은 host 단계표의 최댓값에서 멈춘다.
- 내장 기본값과 상속 결과에 Fable이 나오지 않는다. 사용자가 `set`으로 Fable을 명시하는 것은 허용한다.
- 설정 대상 host는 claude와 codex다. omo는 내장 기본값만 쓴다.

### Must NOT Have

- `~/.claude/agents/`, `~/.codex/agents/`, `~/.codex/config.toml`을 쓰거나 고치지 않는다.
- 이미 봉인된 Orca binding의 owner 값을 설정 변경으로 바꾸지 않는다.
- `model show/resolve/set`이 claude나 codex 프로세스를 실행하지 않는다. 카탈로그 확인은 파일 읽기뿐이다.
- 설정 파일이나 host CLI가 없어도 install·readiness·self-verify가 실패하지 않는다. 새 readiness gate를 만들지 않는다.
- MCP 도구를 추가하지 않는다. 스킬은 CLI를 쓴다.
- env 층(`ISSUEOPS_*`)을 이번에 만들지 않는다.

## Verification Strategy

- Test decision: TDD. Go `testing` 패키지와 기존 golden 체계를 쓴다.
- QA: 모든 작업에 에이전트가 실행하는 시나리오를 둔다. 사람이 손으로 확인하는 항목은 없다.
- Evidence: `.issueops/evidence/task-{N}-{slug}.{ext}` (git ignore 대상)

## Execution Strategy

### Parallel Execution Waves

- Wave 1: T1(해석기), T2(설정 adapter)는 계약 타입만 공유한다. T1의 contract 타입을 먼저 확정한 뒤 병렬로 진행할 수 있다.
- Wave 2: T3(CLI), T4(소비 지점 연결), T5(Orca 주입)는 T1·T2에 의존하고 서로 독립이다.
- Wave 3: T6(스킬), T7(문서·ADR)은 T3~T5의 최종 표면을 문서화한다. T8(golden·전체 검증·실측)은 마지막이다.

이 작업은 한 owner가 순서대로 진행해도 된다. 병렬화는 선택이다.

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|---|---|---|---|
| T1 | — | T3, T4, T5 | T2 |
| T2 | T1 contract 타입 | T3, T4, T5 | T1 |
| T3 | T1, T2, T5(`--agents`) | T6, T8 | T4 |
| T4 | T1, T2 | T8 | T3, T5 |
| T5 | T1, T2 | T3(`--agents`), T8 | T4 |
| T6 | T3 | T8 | T7 |
| T7 | T3, T4, T5 | T8 | T6 |
| T8 | T3~T7 | F | — |

## TODOs

- [ ] 1. 해석기와 내장 기본값 (domain + contract)
  - What to do:
    - `internal/contract/agentmodel/`에 타입을 둔다: `Role`(7개 상수), `Layer{Model, Effort string}`, `Config{Version int; Hosts map[host]map[role]Layer}`, `Resolution{Host, Role, Model, Effort, ModelSource, EffortSource string}`. JSON 필드는 snake_case다.
    - `internal/domain/agentmodel/`에 `Builtin(host, role) (Layer, bool)`, `Resolve(ResolveInput) (Resolution, error)`를 둔다. `ResolveInput`은 Host, Role, Tier, Round, Flag Layer, Local Config, Global Config를 받는다. 소스 표기는 `flag|local|global|default|inherited`다.
    - 상속(`child-implement`→`implement`), docs-only 하향(내장 기본값일 때만), round ≥ 3 상향, 단계표 최댓값 캡을 구현한다. `review-escalate`를 직접 resolve하면 `diff-review`에 round 3을 적용한 결과다.
    - host별 effort 단계표(claude: low→medium→high→xhigh→max, codex: minimal→low→medium→high→xhigh→max, omo: 기존 목록)를 domain으로 옮긴다. `internal/adapter/hostprotocol/argv.go:10-14`의 `supportedEfforts`는 domain 함수를 쓰게 바꾼다.
    - `ValidateSetting(host, role, layer) (warnings []string, err error)`: 설정 가능한 host는 claude와 codex다. 역할은 7개, effort는 단계표 안이어야 한다. 모델 경고는 순수 규칙이다. claude는 별칭(opus, sonnet, haiku, fable) 또는 `claude-` 접두가 아니면 경고한다. codex는 호출자가 넘긴 카탈로그 slug 목록에 없으면 경고하고, 목록이 비면 경고하지 않는다.
    - 기존 `ImplementerDefaults`, `PlannerDefaults`, `ResearchDefaults`, `ReviewEffortForTier`, `Implementer*` 상수는 제거하고 호출부를 해석기로 옮긴다. 남은 참조는 `git grep`으로 0개를 확인한다.
  - Must NOT do: 파일 I/O, 환경변수 읽기. 내장 기본값에 Fable.
  - Recommended Agent: deep; Reason: 상속·티어·라운드 규칙의 조합 테스트.
  - Parallelization: YES; Wave 1; Blocks T3, T4, T5; Blocked By 없음
  - References: `internal/domain/agentmodel/defaults.go:1-75`(대체 대상), `internal/domain/agentmodel/review_effort_test.go:9-22`(티어 규칙 테스트 형식), `internal/adapter/hostprotocol/argv.go:10-14`(effort 목록)
  - Acceptance Criteria: `go test ./internal/domain/agentmodel/... -count=1` 통과. 표 기반 테스트가 우선순위 4단, 필드 병합, child 상속, docs-only(기본값일 때만), round 3/4/5와 max 캡, 잘못된 host·역할·effort 거부, 기본값에 "fable" 부재를 모두 덮는다.
  - QA Scenarios:
    - Happy: Channel shell. `go test ./internal/domain/agentmodel -run TestResolve -v -count=1`. Expected: local effort만 있고 global model만 있으면 결과가 (global model, local effort)이고 소스가 (global, local)이다. Evidence: `.issueops/evidence/task-1-resolver.txt`
    - Failure: 같은 명령의 `TestValidateSetting`. Expected: `host=omo`, `role=reviewer`, `effort=ultra`(claude) 각각 에러다. Evidence: `.issueops/evidence/task-1-resolver-error.txt`
  - Commit: YES; Message: `feat(agentmodel): resolve role models across flag, local, global, and defaults`; Files: `internal/contract/agentmodel/*`, `internal/domain/agentmodel/*`, `internal/adapter/hostprotocol/argv.go`

- [ ] 2. 설정 파일 adapter
  - What to do:
    - `internal/adapter/outbound/agentmodelconfig/`를 만든다.
    - 경로: `GlobalPath(env, home)`는 위 Must Have 규칙이다. `LocalPath(repo)`는 `git -C repo rev-parse --path-format=absolute --git-common-dir`을 실행하고 `repoidentity.SourceRoot`(`internal/domain/repoidentity/source.go:10-30`)로 메인 워크트리를 구한 뒤 `.issueops/agent-models.local.json`을 붙인다. git 저장소가 아니면 local은 "없음"이고 `set --scope local`은 에러다.
    - 읽기: `encoding/json` Decoder에 `DisallowUnknownFields`를 켜고 `{"version": 1, "<host>": {"<role>": {"model": "...", "effort": "..."}}}` 형태만 받는다. version이 1이 아니거나 형식이 다르면 파일 경로와 함께 에러다. 파일이 없으면 빈 Config다.
    - 쓰기: `json.MarshalIndent`(두 칸 들여쓰기, 끝 줄바꿈)로 렌더한다. Go map 키 정렬로 출력이 결정적이다. 같은 디렉터리의 임시 파일에 쓰고 rename한다.
    - exclude: local 파일을 쓸 때 `<common-dir>/info/exclude`에 `/.issueops/agent-models.local.json` 줄이 없으면 추가한다. 디렉터리가 없으면 만든다.
    - Codex 카탈로그: `$CODEX_HOME/models_cache.json`(기본 `~/.codex`)에서 `models[].slug`를 읽는다. 파일이 없거나 형식이 다르면 빈 목록이다. 에러로 만들지 않는다.
  - Must NOT do: host CLI 실행, `~/.codex/config.toml` 수정, 대상 repo `.gitignore` 수정.
  - Recommended Agent: deep; Reason: git·파일시스템 경계.
  - Parallelization: YES; Wave 1; Blocks T3, T4, T5; Blocked By T1 contract 타입
  - References: `internal/adapter/outbound/authority/scope.go:59-82`(GIT_* 환경을 지우고 rev-parse를 실행하는 패턴), `internal/adapter/issueops/cycle_record_store.go:30-39`(CanonicalRepo), `internal/adapter/codex/install_config.go:13-59`(백업·0600 쓰기 패턴), `internal/adapter/outbound/state/statedir.go:10-24`(환경변수 기반 경로 결정 형식)
  - Acceptance Criteria: `go test ./internal/adapter/outbound/agentmodelconfig/... -count=1` 통과. 임시 git 저장소와 `git worktree add`로 만든 연결 워크트리에서 LocalPath가 메인 워크트리 경로를 돌려준다. exclude 등록이 멱등이다. 왕복(read→write→read)이 같은 Config다.
  - QA Scenarios:
    - Happy: Channel shell. 테스트가 `t.TempDir()`에 저장소와 연결 워크트리를 만들고, 연결 워크트리 경로로 local을 쓴다. Expected: 파일이 메인 워크트리 `.issueops/`에 생기고, 두 체크아웃의 `git status --short`가 비어 있다. Evidence: `.issueops/evidence/task-2-config.txt`
    - Failure: `{"version":1,"claude":{"implement":{"modle":"x"}}}`. Expected: 파일 경로와 `unknown field "modle"`이 포함된 에러다. Evidence: `.issueops/evidence/task-2-config-error.txt`
  - Commit: YES; Message: `feat(agentmodelconfig): read and write global and main-worktree model settings`; Files: `internal/adapter/outbound/agentmodelconfig/*`

- [ ] 3. `issueops model` CLI
  - What to do:
    - root runner로 등록한다(`issueops docs` 패턴). `cmd/issueops/issueopsapp/root_command_facade.go:39-66`의 `Runners`에 `"model"`, `internal/contract/cli/commands.go:9-40`에 설명, `internal/adapter/inbound/catalog/cli/usage.go:15-92`에 usage를 추가한다. 구현은 새 leaf 패키지 `cmd/issueops/modelcli/`에 둔다.
    - 하위 명령:
      - `show [--host claude|codex] [--repo PATH] [--json]`: host별 7개 역할의 최종 값과 소스를 출력한다.
      - `set --scope global|local --host H --role R [--model M] [--effort E] [--repo PATH] [--json]`: model과 effort 중 하나 이상이 필요하다. 결과에 `path`, `warnings`, `excluded`(local일 때)를 넣는다.
      - `unset --scope global|local --host H --role R [--field model|effort] [--repo PATH] [--json]`: 필드를 지정하지 않으면 그 역할 항목 전체를 지운다. 비어 있는 host 표는 지운다.
      - `resolve --host H --role R [--tier T] [--round N] [--model M] [--effort E] [--agents] [--repo PATH] --json`: `Resolution`과 함께 `argv`를 낸다. argv는 claude `["claude","-p","--model",M,"--effort",E]`, codex `["codex","exec","-m",M,"-c","model_reasoning_effort=E"]`이고 hostprotocol의 순수 함수가 만든다. omo는 argv를 비워 둔다. `--agents`를 주면 `agent_args`(실행 시점 주입 인자 목록)를 함께 낸다. 이 값은 T5의 `RoleAgentArgs`가 만들며, Orca·cmux 주입과 같은 함수다. codex는 이때 state dir에 role 파일을 쓴다(내용 주소라 같은 설정이면 다시 쓰지 않는다). `--agents` 없이는 파일을 쓰지 않는다.
    - `--repo` 기본값은 cwd다. 종료 코드는 입력 오류 2, 파일·git 오류 1이다(`.issueops/conventions/cli-mcp-and-output.md` §3/§4).
  - Must NOT do: lifecycle catalog 등록, MCP 등록, host CLI 실행.
  - Recommended Agent: deep; Reason: CLI 표면과 golden 계약.
  - Parallelization: YES; Wave 2; Blocks T6, T8; Blocked By T1, T2
  - References: `cmd/issueops/basiccli/docs_cli.go:8-32`(flag set과 JSON 출력 패턴), `cmd/issueops/issueopsapp/cli_facade.go:20`, `internal/adapter/inbound/catalog/cli/usage_test.go:8-14`(usage 포함 검사)
  - Acceptance Criteria: `go test ./cmd/issueops/modelcli/... ./internal/adapter/inbound/catalog/cli/... -count=1` 통과. `go build -o bin/issueops ./cmd/issueops` 후 `ISSUEOPS_STATE_DIR`과 `XDG_CONFIG_HOME`을 임시 디렉터리로 둔 smoke가 통과한다.
  - QA Scenarios:
    - Happy: Channel shell. `XDG_CONFIG_HOME=$T ./bin/issueops model set --scope global --host codex --role research --model gpt-6-luna --effort low --json` 뒤 `./bin/issueops model resolve --host codex --role research --json`. Expected: `model=gpt-6-luna`, `effort=low`, `model_source=global`, `argv[0]=codex`. Evidence: `.issueops/evidence/task-3-cli.json`
    - Agents: `XDG_CONFIG_HOME=$T ISSUEOPS_STATE_DIR=$T/state ./bin/issueops model resolve --host codex --role research --agents --json`. Expected: `agent_args`에 `-c`와 `agents.issueops-research.config_file="$T/state/agent-roles/<hash>.toml"`가 있고 그 파일의 `model`이 `gpt-6-luna`다. Evidence: `.issueops/evidence/task-3-cli-agents.json`
    - Failure: `./bin/issueops model set --scope global --host claude --role implement --effort ultra --json`. Expected: 종료 코드 2, 에러에 허용 effort 목록이 나온다. 깨진 global 파일(`{"version":2}`)로 `model show`를 실행하면 종료 코드 1과 파일 경로가 나온다. Evidence: `.issueops/evidence/task-3-cli-error.json`
  - Commit: YES; Message: `feat(cli): add issueops model show, set, unset, and resolve`; Files: `cmd/issueops/modelcli/*`, `cmd/issueops/issueopsapp/root_command_facade.go`, `internal/contract/cli/commands.go`, `internal/adapter/inbound/catalog/cli/usage.go`

- [ ] 4. 소비 지점 연결 (prepare, next, owner policy, owner 프롬프트)
  - What to do:
    - prepare: `normalizeOwnerDefaults`(`prepare.go:373-385`)가 주입된 port `OwnerDefaults(host, role, repo)`를 쓴다. role은 `snapshot.Record.Delegation != nil`이면 `child-implement`, 아니면 `implement`다. 이를 위해 호출을 snapshot 로드 뒤로 유지하고(`prepare.go:34-39`) record를 넘긴다. 명시 `--owner-model/--owner-effort`는 그대로 우선한다.
    - next: `ports.PlannerDefaults`와 `ports.ReviewEffortForTier`(`internal/application/issueopsnext/ports.go:25,30`)를 `ReviewModel(host, role, tier, repo)` 하나로 바꾼다. phase rank가 implement 미만이면 `plan-review`, 이상이면 `diff-review`다. 티어 하향은 해석기가 맡는다. `service.go:51-55`와 `112-137`을 그에 맞게 바꾼다. port가 에러를 돌려주면 `warnings`에 `agent model settings are invalid: <path>: <원인>`을 남기고 `review.model`과 `review.effort`를 비운다. tier와 lenses는 그대로 계산한다. `Review` 계약 타입(`internal/contract/issueopsnext/types.go:70-82`)은 바꾸지 않는다.
    - prepare에서 설정 해석이 실패하면 record·intent·worktree를 바꾸기 전에 파일 경로가 담긴 에러로 끝낸다.
    - owner policy: `PolicyContext`(`internal/application/issueopsowner/policy.go:15-25`)가 reviewer를 `diff-review`, research를 `research`, 새 `ReaderCheckModel/Effort`를 `reader-check`로 해석한다.
    - owner 프롬프트: `internal/adapter/issueops/testdata/execution_owner_prompt.txt`의 research 줄 옆에 `- reader_check_model={READER_CHECK_MODEL} ({READER_CHECK_EFFORT})` 한 줄을 추가하고, `.issueops/prompt-engineering/prompts/issueops-v1-owner-execution-v1.md`의 PROMPT 블록과 자리값 표를 같은 바이트로 맞춘다. `owner_context.go:46-48,93,102`에 키를 추가한다. 라운드 상향 문장(`execution_owner_prompt.txt:165-168`)은 `issueops model resolve --round N` 사용으로 바꾼다.
    - composition root(`cmd/issueops/issueopsapp/issueops_next_wiring.go:57-59` 등)가 adapter를 읽어 port를 주입한다. 주입이 없으면 구조화된 오류를 낸다(기본 구현을 두지 않는다).
  - Must NOT do: resume 경로에서 설정을 다시 읽어 binding을 바꾸는 것. `Review` JSON 키 변경.
  - Recommended Agent: deep; Reason: 봉인·resume 계약과 프롬프트 parity.
  - Parallelization: YES; Wave 2; Blocks T8; Blocked By T1, T2
  - References: `internal/application/issueopspreparation/orca_begin.go:33`(probe 일치 검사), `internal/application/issueopsnext/review_tier_test.go`, `internal/adapter/issueops/execution_owner_packet_test.go:498-541,596`, `internal/adapter/issueops/execution_owner_prompt_parity_test.go:13`
  - Acceptance Criteria: `go test ./internal/application/... ./internal/adapter/issueops/... ./cmd/issueops/issueopscli/... -count=1` 통과. 기존 resume 테스트가 수정 없이 통과한다.
  - QA Scenarios:
    - Happy: Channel shell. 테스트에서 local 설정 `claude.diff-review.effort=xhigh`를 주고 implement phase record로 next를 계산한다. Expected: `review.model=claude-opus-5-5`, `review.effort=xhigh`, docs-only 티어여도 xhigh다. Evidence: `.issueops/evidence/task-4-next.txt`
    - Failure: 깨진 global 파일(`{"version":1,"claude":{"implement":{"modle":"x"}}}`)로 next를 계산한다. Expected: 종료 코드 0, `warnings`에 파일 경로가 있고 `review.model`이 비어 있다. 같은 파일로 prepare preview를 실행하면 에러이고 record가 바뀌지 않는다. Evidence: `.issueops/evidence/task-4-broken-config.txt`
    - Edge: Delegation이 있는 record로 prepare preview. Expected: `child-implement` 설정이 없으면 implement 값을 쓰고, 설정이 있으면 그 값을 쓴다. 이미 Orca binding이 있는 record의 resume은 설정을 바꿔도 binding 값을 쓴다. Evidence: `.issueops/evidence/task-4-prepare.txt`
  - Commit: YES; Message: `feat(issueops): resolve owner, review, research, and reader models from settings`; Files: 위 경로

- [ ] 5. owner 실행 명령의 역할 에이전트 주입 (Orca, cmux)
  - What to do:
    - hostprotocol에 순수 함수 두 개를 둔다. `ClaudeAgentsJSON([]RoleAgent) (string, error)`는 `{"issueops-<role>": {"description": "...", "prompt": "...", "model": M, "effort": E}}`를 만든다. `CodexRoleFile(RoleAgent) string`은 `name`, `description`, `model`, `model_reasoning_effort`, `developer_instructions` TOML을 만든다. 주입 역할은 plan-review, diff-review, review-escalate, research, reader-check 다섯 개다.
    - 주입 인자 생성은 한 함수가 소유한다. application에 `RoleAgentArgs(host, repo) ([]string, error)` port를 두고, composition root가 해석기·설정 adapter·Codex role 파일 쓰기 adapter를 조립해 주입한다. claude면 `["--agents", <JSON>]`, codex면 state dir `agent-roles/<sha256>.toml`에 파일을 쓰고(내용 주소라 경쟁이 없다) 역할마다 `["-c", "agents.issueops-<role>.config_file=\"<path>\""]`를 돌려준다. omo는 빈 목록이다. T3의 `resolve --agents`, Orca, cmux가 모두 이 함수를 쓴다.
    - Orca owner terminal을 invoke하는 진입점은 세 곳이다. 세 곳 모두 stage가 terminal 생성일 때 `RoleAgentArgs`를 `MarkInvoking` **전에** 호출하고, 결과를 invoke 요청에 싣는다.
      1. prepare: `advanceOrca`(`internal/application/issueopspreparation/prepare.go:203-237`). `intentRequest`(`:204`) 뒤, `MarkInvoking`(`:233`) 앞이다. 결과는 `preparationcontract.IntentRequest`의 새 필드 `RoleAgentArgs []string`에 싣는다.
      2. resume: `BeginIntent`(`internal/application/issueopslease/resume.go:88`) **앞**이다. `BeginIntent`가 pending intent를 먼저 기록하고(그 기록은 실패해도 롤백되지 않는다), pending이 있으면 같은 resume 명령은 `run execution reconcile`로 거부되기 때문이다(`internal/domain/issueopslease/resume.go:87-88`). 조건은 첫 stage가 terminal일 때(`plan.ReusedTerminalPTYID == ""`)이고, host와 repo는 `snapshot.Record.Stable.Execution.Orca`의 `OwnerHost`와 record repo에서 가져온다(`resume_begin.go:40`이 같은 값을 `Probe.Host`로 복사한다). 계산한 인자는 루프의 terminal stage invoke(`resume.go:114-119`)에 넘긴다.
      3. reconcile: `internal/application/issueopslease/reconcile.go:98`의 `MarkInvoking` 앞이다. host와 repo는 sealed intent의 `Probe.Host`·`Probe.Repo`에서 가져온다. prepare가 만든 intent는 dispatch stage 전까지 Orca binding이 없으므로(`internal/application/issueopspreparation/orca_receipt.go:93`) binding을 쓰지 않는다. repository가 `Stage`를 채우는 자리(`internal/adapter/outbound/issueopslease/reconcile_repository.go:79-80`, `resume_repository.go:181`)에서 `ProbeHost`·`ProbeRepo`를 `ReconcileIntentState`·`ResumeIntentState`에 함께 싣는다. Orca terminal 생성이 쓰는 host도 같은 `Probe.Host`다(`internal/adapter/orca/execution.go:388`).
      resume과 reconcile의 `stages.Invoke(ctx, intent)`는 wiring(`cmd/issueops/issueopsapp/issueops_resume_wiring.go:143-150`, `issueops_reconcile_wiring.go:107-108`)에서 `IntentRequestBuilder.Build`로 요청을 다시 만든다. 이 port의 시그니처를 `Invoke(ctx, intent, roleAgentArgs []string)`로 넓혀 계산한 인자를 요청에 붙인다. 필드는 intent에 저장하지 않으므로 intent codec과 권한 검증(`intent_request.go:22-25`)은 바뀌지 않는다.
      해석이 실패하면 prepare와 reconcile은 `MarkInvoking` 없이 에러를 돌려준다. intent는 `not_invoked_proven`으로 남고 `invocation_attempts`가 늘지 않으며, 설정을 고친 뒤 같은 명령으로 재시도할 수 있다. resume은 `BeginIntent` 전에 실패하므로 pending intent를 남기지 않고, 같은 resume 명령으로 재시도할 수 있다. 세 서비스는 같은 application port 하나를 주입받는다.
    - Orca adapter: 요청의 인자를 `port.Orca` 생성 요청(`internal/port/orca.go:149` 부근)의 `ExtraArgs []string`으로 넘긴다. `ownerAgentCommand`(`client_decode.go:226-260`)는 각 인자에 `shellSingleQuote`를 정확히 한 번 적용해 덧붙인다. 빈 값 인자는 렌더하지 않는다.
    - cmux: `BuildInteractiveArgv`(`argv.go:19-59`)에 `extra []string`을 받아 `--` 앞에 덧붙이게 한다. `issueops_cmux_wiring.go:42`의 `PrepareLauncher` 호출 전에 composition root가 `RoleAgentArgs`를 불러 넘긴다. 프로필 검증 호출(`:55`)은 빈 목록을 넘긴다.
    - `Probe`의 claude help 검사(`internal/adapter/orca/client.go:237-247`)에 `--agents`를 추가한다.
  - Must NOT do: probe 봉인 필드(host/model/effort)나 intent 기록에 역할 에이전트를 넣는 것. 인용 중첩. `MarkInvoking` 뒤에 설정을 읽는 것.
  - Recommended Agent: deep; Reason: 셸 인용, intent 상태 분류, 외부 launcher 계약.
  - Parallelization: YES; Wave 2; Blocks T3의 `--agents`, T8; Blocked By T1, T2
  - References: `internal/adapter/orca/client_decode.go:226-262`, `internal/adapter/orca/client.go:222-247,554-583`, `internal/adapter/outbound/issueopspreparation/orca.go:85-91`, `internal/application/issueopspreparation/prepare.go:204-240`, `internal/application/issueopslease/resume.go:110-125`, `internal/application/issueopslease/reconcile.go:95-105`, `cmd/issueops/issueopsapp/issueops_resume_wiring.go:143-150`, `cmd/issueops/issueopsapp/issueops_reconcile_wiring.go:96-108`, `cmd/issueops/issueopsapp/issueops_cmux_wiring.go:42-55`, `.issueops/cautions/issueops-orchestration.md`("Orca create 호출의 모호한 실패를 재시도하지 말 것" 절의 인용 규칙)
  - Acceptance Criteria: `go test ./internal/adapter/orca/... ./internal/adapter/hostprotocol/... ./internal/application/issueopspreparation/... ./internal/application/issueopslease/... ./cmd/issueops/issueopsapp/... -count=1` 통과. Orca 명령 문자열을 `SplitCommandTokens`로 다시 나눴을 때 `--agents` 다음 토큰이 유효한 JSON이고, 전체 명령 길이가 8 KiB 미만이다. cmux argv에 같은 주입 인자가 `--` 앞에 있다.
  - QA Scenarios:
    - Happy: Channel shell. claude와 codex 각각 ownerAgentCommand golden 문자열과 cmux argv 비교. Expected: claude는 `--agents '<json>'`, codex는 `-c 'agents.issueops-research.config_file="/…/agent-roles/<hash>.toml"'`가 정확히 한 번 인용되고, cmux argv에도 같은 인자가 들어간다. Evidence: `.issueops/evidence/task-5-inject.txt`
    - Resume: resume이 terminal stage를 invoke할 때 만든 요청에 주입 인자가 들어간다. reconcile도 같다. Orca binding이 없는 prepare intent를 terminal stage에서 reconcile하면 sealed `Probe.Host`에 맞는 주입 인자가 실린다. Evidence: `.issueops/evidence/task-5-resume.txt`
    - Failure: 깨진 local 설정으로 prepare, resume, reconcile의 terminal stage를 각각 진행한다. Expected: prepare와 reconcile은 Orca `Invoke`가 호출되지 않고 intent의 `invocation_state`가 `not_invoked_proven`이며 `invocation_attempts`가 늘지 않는다. resume은 새 pending intent를 남기지 않는다. 세 경우 모두 설정을 고친 뒤 같은 명령이 진행된다. description에 작은따옴표가 들어간 RoleAgent도 인용 왕복 결과가 원문과 같다. Evidence: `.issueops/evidence/task-5-inject-error.txt`
  - Commit: YES; Message: `feat(issueops): inject role agents into Orca and cmux owner sessions`; Files: `internal/adapter/hostprotocol/*`, `internal/adapter/orca/*`, `internal/port/orca.go`, `internal/contract/issueopspreparation/*`, `internal/application/issueopspreparation/*`, `internal/application/issueopslease/*`, `internal/adapter/outbound/issueopslease/*`, `cmd/issueops/issueopsapp/issueops_resume_wiring.go`, `cmd/issueops/issueopsapp/issueops_reconcile_wiring.go`, `cmd/issueops/issueopsapp/issueops_cmux_wiring.go`, composition root

- [ ] 6. 스킬
  - What to do:
    - `skills/io-model/SKILL.md`와 `skills/io-model/agents/openai.yaml`을 만든다. 스킬은 대화 요청을 `issueops model set|unset`으로 옮기고 결과를 `issueops model show --json`으로 다시 읽어 보여 준다. 모델 이름 표를 본문에 복사하지 않는다.
    - `skills/issueops-review/SKILL.md:159-173`의 라운드 상향 산문을 `issueops model resolve --host "$HOST" --role diff-review|plan-review --round "$ROUND" --json`의 `argv` 사용으로 바꾼다. Orca가 띄운 세션에서는 `issueops-review-escalate` 서브에이전트를 쓸 수 있다고 적는다. stale Fable 인용을 ADR 인용으로 바꾼다.
    - `skills/issueops-remote-write/SKILL.md`의 독자 검토 단계(:49 부근)가 `reader-check` 역할을 쓰게 한다.
    - `skills/issueops/references/execution.md:360-380`의 기본값 표를 `issueops model show` 안내로 바꾼다. `skills/issueops/references/session-choice.md:358-384`의 Herdr 실행에 `issueops model resolve --role implement` 값을 쓰도록 적는다.
  - Must NOT do: 스킬 본문에 모델 ID 표 복사.
  - Recommended Agent: quick; Reason: 문서 편집과 검증 스크립트.
  - Parallelization: YES; Wave 3; Blocks T8; Blocked By T3
  - References: `skills/io-update/`(io- 접두 스킬 형식), `.issueops/cautions/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md`
  - Acceptance Criteria: `python3 scripts/validate-skill.py skills/io-model skills/issueops-review skills/issueops-remote-write skills/issueops` 통과, `python3 scripts/verify-skill-shell.py` 통과, `go test ./internal/adapter/skillcontract/... -count=1` 통과.
  - QA Scenarios:
    - Happy: Channel shell. 위 validate 명령. Expected: 종료 코드 0. Evidence: `.issueops/evidence/task-6-skills.txt`
    - Failure: `git grep -n "gpt-6-astra\|claude-opus-5-5" skills/`. Expected: io-model과 issueops-review 본문에 모델 ID 표가 없다(예시 명령의 인자 한두 개는 허용). Evidence: `.issueops/evidence/task-6-skills-grep.txt`
  - Commit: YES; Message: `feat(skills): add io-model and route review and reader roles through issueops model`; Files: `skills/io-model/*`, `skills/issueops-review/SKILL.md`, `skills/issueops-remote-write/SKILL.md`, `skills/issueops/references/*`

- [ ] 7. 문서와 ADR
  - What to do:
    - 새 ADR: 역할별 모델 설정, 경로, 우선순위(env 층은 비워 둠), local 위치를 `.issueops/` 아래 두는 근거(git이 무시하는 파일은 docs layout 검사 대상이 아님), 네이티브 에이전트 파일 기각, Fable 규칙 유지, Codex 리뷰 effort high, Claude implement opus. `adr/2026-09-24-claude-role-models-...md`는 이 ADR로 부분 대체된다고 표시한다.
    - `.issueops/conventions/state-policy-and-hooks.md` §6과 `.issueops/architecture/runtime.md` 설정 위치 표를 갱신한다.
    - `.issueops/architecture/issueops.md` 기본값 표, `.issueops/architecture/host-integration.md` 스킬 수, `.issueops/OPERATIONS.md:49` 스킬 목록, `README.md:175-184`·`README.en.md:282` 표와 목록을 갱신한다.
    - 문서 단계(issueops-docs)에서 `project_docs` MCP 계약으로 반영하고 문서 검사를 실행한다.
  - Must NOT do: 실제 코드와 다른 값을 적는 것.
  - Recommended Agent: quick; Reason: 문서 갱신.
  - Parallelization: YES; Wave 3; Blocks T8; Blocked By T3, T4, T5
  - References: `.issueops/ADR.md`, `.issueops/CAUTIONS.md` Update workflow
  - Acceptance Criteria: `uv run --directory skills/project-docs-optimize python -m scripts.check --root "$PWD" --mode check --json`가 위반 0이다.
  - QA Scenarios:
    - Happy: 위 검사 명령. Expected: `ok: true`. Evidence: `.issueops/evidence/task-7-docs.json`
    - Failure: `git grep -n "PlannerDefaults\|ImplementerDefaults\|ResearchDefaults"`. Expected: 0건이다. Evidence: `.issueops/evidence/task-7-stale.txt`
  - Commit: YES; Message: `docs(issueops): record role model settings and their precedence`; Files: `.issueops/**`, `README*.md`

- [ ] 8. golden 갱신, 전체 검증, 실측
  - What to do:
    - 의도된 계약 변경이므로 golden을 갱신한다: `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -update -count=1`, `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -update -count=1`, `go test ./internal/architecture -run TestDDDResponsibilityInventoryMatchesSource -update-ddd-inventory -count=1`. 각 diff를 읽고 의도한 변경만 있는지 확인한다.
    - 전체 검증: `.issueops/testing/unit-and-contract.md`의 Go 기본 검증 목록과 `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json`, `python3 scripts/python_suite_runner.py`.
    - 실측: 빌드한 `bin/issueops`로 (a) claude: `issueops model resolve --host claude --role research --json`의 값으로 만든 `--agents` JSON을 `claude -p --model haiku --agents ...`에 넣고 `issueops-research` 서브에이전트에게 자기 model ID를 보고하게 한다. (b) codex: research를 `gpt-6-luna / low`로 둔 role 파일로 `codex exec -m gpt-6-luna -c model_reasoning_effort=medium -c agents.issueops-research.config_file=...`를 실행하고 `~/.codex/state_5.sqlite`의 `thread_spawn_edges`·`threads`에서 자식 스레드 model과 effort를 읽는다. 실측은 scratch 디렉터리에서 하고 저장소를 바꾸지 않는다.
  - Must NOT do: 실패한 단계 뒤 부분 결과로 완료 주장.
  - Recommended Agent: deep; Reason: 전체 battery와 외부 host 실측.
  - Parallelization: NO; Wave 3 마지막; Blocks F; Blocked By T3~T7
  - References: `.issueops/TESTING.md` 최소 완료 기준, `.issueops/testing/self-verification.md`
  - Acceptance Criteria: 아래 G7~G9.
  - QA Scenarios:
    - Happy: 위 battery. Expected: 모두 종료 코드 0. Evidence: `.issueops/evidence/task-8-battery.txt`
    - Failure/edge: 실측 (b)에서 부모를 `gpt-5.6-terra`처럼 다른 모델로 두는 대신, 플랜 제약상 부모와 자식 effort를 다르게 둔다(부모 medium, 자식 low). Expected: 자식 effort가 low로 기록된다. Evidence: `.issueops/evidence/task-8-live-codex.txt`
  - Commit: YES(golden 갱신분); Message: `test(contract): refresh goldens for issueops model`; Files: `cmd/issueops/testdata/*`, `internal/architecture/testdata/*`

## 게이트 (G1..G9)

- G1 CHECK `go test ./internal/domain/agentmodel/... -count=1` EXPECT 종료 코드 0, 우선순위·병합·상속·티어·라운드·Fable 부재 테스트 포함
- G2 CHECK `go test ./internal/adapter/outbound/agentmodelconfig/... -count=1` EXPECT 종료 코드 0, 연결 워크트리에서 메인 워크트리 local 해석과 exclude 멱등 테스트 포함
- G3 CHECK `go test ./cmd/issueops/modelcli/... ./internal/adapter/inbound/catalog/cli/... -count=1` EXPECT 종료 코드 0
- G4 CHECK `go test ./internal/application/... ./internal/adapter/issueops/... ./cmd/issueops/issueopscli/... -count=1` EXPECT 종료 코드 0, 기존 resume 테스트 무수정 통과
- G5 CHECK `go test ./internal/adapter/orca/... ./internal/adapter/hostprotocol/... ./internal/application/issueopspreparation/... ./internal/application/issueopslease/... ./cmd/issueops/issueopsapp/... -count=1` EXPECT 종료 코드 0, 인용 왕복, 8 KiB 미만, cmux argv 주입, resume·reconcile 요청 주입, binding 없는 prepare intent의 reconcile이 `Probe.Host`로 주입, 깨진 설정에서 prepare·reconcile은 intent `not_invoked_proven` 유지와 resume은 pending 미생성 테스트 포함
- G6 CHECK `python3 scripts/validate-skill.py skills/io-model skills/issueops-review skills/issueops-remote-write skills/issueops && python3 scripts/verify-skill-shell.py` EXPECT 종료 코드 0
- G7 CHECK `gofmt -l $(git ls-files '*.go')`가 빈 출력이고 `go vet ./...`, `go test ./... -count=1`, `go test -race ./... -count=1`, `go build -o bin/issueops ./cmd/issueops`, `GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...`(GOOS=linux 포함), `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json` EXPECT 하나의 run에서 전부 통과
- G8 EXPECT 실측: `issueops model resolve --agents`가 돌려준 `agent_args`로 띄운 Claude 세션의 서브에이전트가 설정의 전체 model ID로 실행되고, 같은 방식으로 띄운 Codex 세션의 자식 스레드가 role 파일의 model·effort(`gpt-6-luna / low`)로 기록된다. 주입 인자 생성은 Orca·cmux와 같은 함수이므로 이 실측이 세 경로의 인자를 대표한다
- G9 EXPECT canonical worktree 안에서 `./bin/issueops model resolve --host codex --role research --json`의 소스가 메인 워크트리 local이고, local 파일을 만든 뒤에도 메인 워크트리와 canonical worktree의 `git status --short`에 그 파일이 없다

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제3장 아키텍처 원칙: 해석 규칙은 domain, 파일 읽기·쓰기와 git 실행은 adapter, 조합은 application에 둔다.
- `.issueops/conventions/go-and-packages.md` §2 레이어 경계: `cmd/issueops/<cli>`는 flag·출력·dispatch만 맡고, `issueopsapp`만 concrete adapter를 조립한다. `hostprotocol`은 인자 생성만 하고 fs I/O를 하지 않는다. 그래서 Codex role 파일 쓰기는 composition root가 주입한 adapter가 맡는다.
- `.issueops/ARCHITECTURE.md` 의존 방향 불변식과 `internal/architecture/dependency_test.go`: composition root 밖의 adapter import edge를 만들지 않는다.
- `.issueops/conventions/state-policy-and-hooks.md` §6 Config / env: `~/.config/issueops/`는 예약 경로이고 우선순위는 `flag → env → workspace → user → default`로 고정돼 있다. 이 계획은 그 순서를 지키고 env 층을 비워 두며, 조항과 ADR을 갱신한다.
- `.issueops/architecture/runtime.md` 설정 위치 기준: XDG base directory를 우선한다. 그래서 `XDG_CONFIG_HOME`을 따른다.
- `.issueops/CONSTITUTION.md` 제2장 안전 불변식: workspace 밖 파일 접근은 정책으로 드러내야 한다. `~/.config` 쓰기는 `model set --scope global` 명시 명령에서만 한다.
- `.issueops/adr/2026-07-07-standalone-harness-policy.md`: 설정 파일이나 host CLI가 없어도 install·readiness·self-verify가 성공해야 한다. 해석기는 파일이 없으면 내장 기본값을 쓴다.
- `.issueops/adr/2026-09-24-claude-role-models-opus-5-plans-and-reviews-sonnet-5-impleme.md`: Fable은 자동 기본값·폴백·리뷰 상향에 쓰지 않는다. 이 계획은 그 규칙을 해석기 테스트로 고정하고, Claude implement 기본값을 opus로 바꾸는 부분은 새 ADR로 대체한다.
- `.issueops/adr/2026-09-08-adversarial-review-throughput...md`: owner 프롬프트에 tier를 치환하지 않는다(prepare 시점 tier는 항상 default). reader-check 자리값도 tier 없이 기본 effort로 넣는다.
- `.issueops/cautions/integrations.md` §7 MCP schema drift: CLI만 추가하고 MCP는 추가하지 않는다. 이 결정을 ADR에 적는다.
- `.issueops/cautions/issueops-lifecycle.md` §27: 스킬 description이 `response_contracts.golden.json`에 고정되므로 같은 커밋에서 갱신한다.
- `.issueops/cautions/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md`: `skills/io-model/agents/openai.yaml`을 만든다.
- `.issueops/cautions/issueops-orchestration.md`("Orca create 호출의 모호한 실패를 재시도하지 말 것"): payload에 POSIX single-quote 인코더를 정확히 한 번 적용하고 빈 값 flag는 렌더하지 않는다.
- `.issueops/operations/guides/hosts.md`와 `.issueops/testing/issueops-execution.md`: native host argv 64 KiB 예산을 공유하므로 주입 인자를 8 KiB 미만으로 검사한다.
- `.issueops/cautions/2026-08-28-verify-host-cli-flags-against-the-installed-version.md`: claude `--agents`와 codex `-c agents.<name>.config_file=`은 설치 버전(claude 2.1.293, codex-cli 0.160.1)에서 실측했다. Probe에 `--agents` help 검사를 추가하고 argv를 adapter 테스트로 고정한다.
- `.issueops/cautions/2026-10-02-real-host-qa-tool-results-service-tier-amfi-exact-edits.md`: 전역 `~/.codex` 설정을 고치지 않는다. 주입은 `-c` override와 state dir 파일로만 한다.
- `.issueops/cautions/2026-08-03-orca-resume-prompt-template-trust-root.md`: resume은 봉인된 digest와만 비교한다. 새 자리값은 prepare가 봉인하는 값에 포함되고, 이미 봉인된 실행에는 영향을 주지 않는다.
- `.issueops/cautions/integrations.md` §12와 `.issueops/cautions/2026-08-28-install-dry-run-spawned-the-claude-cli.md`: preview·resolve 경로에서 외부 CLI를 실행하지 않는다.
- `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md`: 제거하는 agentmodel 함수 이름을 Go, golden, testdata, skills, `.issueops`, configs에서 `git grep`으로 0건 확인한다.
- `.issueops/TESTING.md` 최소 완료 기준: 완료 증거는 한 번의 "전 단계 통과" run에서 나와야 한다.

## 재사용하는 기존 구현

- `internal/domain/agentmodel`: 패키지를 그대로 확장한다. 새 패키지를 만들지 않는다. 기존 소비자 세 곳(`issueops_next_wiring.go:57-59`, `issueopsowner/policy.go:17-18`, `issueopspreparation/prepare.go:377`)이 같은 패키지를 계속 쓴다.
- `repoidentity.SourceRoot`(`internal/domain/repoidentity/source.go:10-30`): 연결 워크트리에서 메인 워크트리를 구하는 순수 함수다. 새로 만들지 않는다.
- `ScopeResolver.gitIdentity`(`internal/adapter/outbound/authority/scope.go:59-82`)의 `GIT_*` 환경 제거와 `rev-parse` 실행 방식을 같은 형태로 따른다. 그 타입은 authority 전용 의존을 갖고 있어 직접 쓰지 않는다.
- `encoding/json`: 설정 파일 읽기·쓰기에 쓴다(`.issueops/TECH_STACK.md:74`).
- `shellSingleQuote`(`internal/adapter/orca/client_decode.go:262`)와 `SplitCommandTokens`: 주입 인자 인용과 왕복 테스트에 쓴다.
- `hostprotocol`의 effort 목록(`argv.go:10-14`): domain으로 옮겨 한 곳에서 관리하고 hostprotocol이 그것을 쓴다.
- `cmd/issueops/basiccli/docs_cli.go`의 root runner 패턴과 `contractgolden`·`response_contract_golden_test.go` golden 체계를 쓴다.
- 새로 만드는 것: Codex 역할 파일 렌더러. 고정 키 다섯 개(`name`, `description`, `model`, `model_reasoning_effort`, `developer_instructions`)를 TOML basic string으로 쓰는 순수 함수다. 문자열은 `strconv.Quote`가 아니라 JSON 이스케이프(`json.Marshal`)로 만들어 TOML basic string과 호환되게 한다. `installutil.TOMLString`(`internal/adapter/installutil/install_util.go:168-174`)과 같은 방식이며, hostprotocol은 installutil에 의존하지 않도록 같은 한 줄 규칙을 쓴다. 범용 TOML 라이브러리는 들이지 않는다.

## 성능 영향

- hot path가 아니다. 해석은 `execution prepare`, `issueops next`, Orca terminal 생성, `issueops model` 실행 때 한 번씩 일어난다.
- 추가 비용: 설정 파일 최대 두 개를 읽고(각 수 KB), `git rev-parse` 한 번을 실행한다. `issueops next`는 지금도 git 관측을 하므로 rev-parse 한 번(수 ms)이 더해진다. 해석 자체는 역할 7 × host 2 상수 표 조회다.
- Codex role 파일은 내용 주소로 쓰므로 같은 설정이면 다시 쓰지 않는다.
- 측정: `issueops next --json`의 실행 시간을 변경 전후로 `hyperfine -w 3 -r 20`(없으면 `time` 20회 평균)으로 재서 증가분을 검증 보고에 적는다. 증가가 20 ms를 넘으면 원인을 보고한다.

## 하위 호환성과 side effect

- **CLI**: `issueops model`이 새로 생긴다. 기존 명령의 flag와 JSON 키는 바뀌지 않는다. `usage.golden.txt`와 `response_contracts.golden.json`(명령 목록, 스킬 목록)이 의도적으로 바뀐다.
- **`issueops next --json`**: `review` 키 구조는 같다. 값이 바뀐다: Codex 리뷰 effort가 xhigh에서 high로, 설정이 있으면 그 값으로 바뀐다.
- **execution prepare**: owner 미지정 시 Claude 기본값이 `claude-sonnet-5-5`에서 `claude-opus-5-5`로 바뀐다. 명시 플래그는 그대로 우선한다.
- **record/lease 스키마**: 바꾸지 않는다. 해석 결과는 기존 `OrcaBinding.OwnerModel/OwnerEffort`에만 들어간다. 새 최상위 record 필드가 없으므로 `TestLeaseRecordCarriesEveryPersistedTopLevelField`에 영향이 없다.
- **owner packet**: `ReaderCheckModel/Effort` 필드를 추가한다. 이미 봉인된 packet은 resume에서 digest로만 비교하고 다시 렌더하지 않는다(`.issueops/cautions/2026-08-03-orca-resume-prompt-template-trust-root.md`). 미확인 가정: packet 디코더가 새 필드 부재를 오류로 보지 않는다. T4에서 기존 packet fixture를 디코드하는 테스트로 확인한다.
- **owner 프롬프트(LLM 프롬프트 본문 변경)**: reader-check 줄 하나 추가와 라운드 상향 문장 교체다. 측정 가능한 출력 기준은 다음과 같다.
  - 실패 기준 1: 렌더된 프롬프트에 `{READER_CHECK_MODEL}` 같은 미치환 자리값이 남는다. `validateExecutionOwnerPromptInputs`와 렌더 테스트로 확인한다.
  - 실패 기준 2: 라운드 상향 지시가 `issueops model resolve --round` 명령을 포함하지 않는다. 프롬프트 parity 테스트와 문자열 검사로 확인한다.
  - 실패 기준 3: owner가 implementation-review 기록에 실제로 넘긴 reviewer model/effort 대신 자리값 문자열을 적는다. 기존 `TestOwnerArtifactsRouteModelRoles` 확장으로 확인한다.
- **파일 side effect**: `model set --scope global`은 `$XDG_CONFIG_HOME/issueops/` 또는 `~/.config/issueops/`에 파일 하나를 쓴다. `--scope local`은 메인 워크트리 `.issueops/agent-models.local.json`과 `<common-dir>/info/exclude` 한 줄을 쓴다. Orca 주입은 state dir `agent-roles/`에 Codex role 파일을 쓴다. 어느 것도 추적 파일을 바꾸지 않는다.
- **외부 host**: Orca owner 명령과 cmux argv가 길어진다(8 KiB 미만으로 검사). 설치된 claude가 `--agents`를 지원하지 않으면 Probe가 terminal 생성 전에 거부한다.
- **Orca intent**: 설정 오류나 role 파일 쓰기 실패는 prepare·reconcile에서는 `MarkInvoking` 전에, resume에서는 `BeginIntent` 전에 끝난다. 그래서 intent는 `not_invoked_proven`으로 남거나 새로 생기지 않고, 재시도 횟수를 쓰지 않는다. `ResumeIntentState`·`ReconcileIntentState`에 `ProbeHost`·`ProbeRepo`가 추가되지만 이는 메모리 안 상태이며 저장 형식은 바뀌지 않는다. 새 `IntentRequest.RoleAgentArgs` 필드는 intent에 저장하지 않으므로 기존 intent 기록 형식은 바뀌지 않는다.
- **설정 오류**: `issueops next`는 경고와 빈 review 값으로 계속 동작하고, prepare와 `issueops model`은 상태를 바꾸기 전에 실패한다.
- **기존 데이터**: 설정 파일이 없는 사용자는 내장 기본값만 바뀐 것 외에 차이가 없다.
- **롤백**: 커밋을 되돌리면 상수 기반 동작으로 돌아간다. 사용자가 만든 설정 파일은 읽히지 않을 뿐 남는다. 봉인된 binding에는 모델 이름만 있으므로 되돌려도 resume이 깨지지 않는다.

## Final Verification Wave

- F1 Plan Compliance Audit: TODO 1~8과 G1~G9가 계획대로 수행됐는지 대조한다.
- F2 Code Quality Review: 죽은 코드, 과한 추상화, 남은 상수 참조가 없는지 issueops-slop-clean과 diff 리뷰로 확인한다.
- F3 Real Manual QA: G8·G9 실측 증거를 남긴다.
- F4 Scope Fidelity Check: omo 설정, MCP 도구, env 층, 네이티브 에이전트 파일이 추가되지 않았는지 확인한다.

## Commit Strategy

TODO별 Commit 항목을 따른다. Conventional Commit 제목과 Lore 본문(`atomic-commit-push`)을 쓴다. golden 갱신은 그 계약을 바꾼 커밋에 포함하거나 T8의 별도 커밋으로 묶는다.

## Success Criteria

이슈 #553의 완료 기준 여섯 개가 G1~G9로 모두 확인되고, 전체 battery가 한 run에서 통과한다.
