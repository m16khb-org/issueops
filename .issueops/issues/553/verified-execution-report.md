# #553 verified-execution report

- lifecycle: `io-e66ba158272a`, branch `553-agent-model-config`, base `3a5383281cba84f3ffdff18448e164a48c52ae83`
- 계획: `.issueops/issues/553/plan.md`(봉인 원본 sha256 `230d4da9…ac4791ac4`)
- 게이트 원장: `.issueops/issues/553/gates.md`
- 실측 기록: `.issueops/issues/553/live-check.md`

## 목표와 수용 기준

이슈 #553의 완료 기준 여섯 개를 계획의 G1~G9로 옮기고, 원장에서는 G1~G16으로 나눴다.
원장 결과는 각 게이트의 EVIDENCE가 정본이다.

| 기준 | 게이트 |
|---|---|
| set/unset/show가 global·local을 쓰고, 워크트리에서도 메인 워크트리 local을 따른다 | G2, G3, G14, G15(local) |
| resolve가 우선순위·필드 병합·docs-only 하향·round 상향을 지키고 출처를 보고한다 | G1, G3 |
| prepare(owner 미지정)·next `.review`·owner 프롬프트가 해석기 값을 쓴다 | G4 |
| Orca·direct(cmux) owner 명령에 역할 에이전트가 주입되고 실제 host에서 역할 모델로 실행된다 | G5, G15(claude·codex) |
| local 파일이 info/exclude에 등록되어 git status에 나오지 않는다 | G2, G14, G15(local) |
| io-model과 관련 스킬이 모델 표를 복사하지 않고 validate를 통과한다 | G6 |
| 전체 battery | G7~G13, G16 |

## 구현 요약

- `internal/contract/agentmodel`, `internal/domain/agentmodel`: host × 역할 해석기(`Resolve`), 내장 기본값,
  effort 단계표, 설정 검증. 기존 `ImplementerDefaults`·`PlannerDefaults`·`ResearchDefaults`·`ReviewEffortForTier`와
  상수는 제거했다.
- `internal/adapter/outbound/agentmodelconfig`: global(XDG)·local(메인 워크트리) 경로, 엄격한 JSON 읽기,
  결정적 쓰기, info/exclude 등록, Codex 모델 캐시 읽기, 내용 주소 role 파일 쓰기.
- `internal/application/agentmodel`: 설정 로드 + 해석, 주입 역할 다섯 개의 해석.
- `issueops model show|set|unset|resolve`(`cmd/issueops/modelcli`).
- 소비 지점: prepare owner 기본값(Delegation이면 child-implement), `issueops next`의 review(plan 전 plan-review,
  implement부터 diff-review, 설정 오류면 경고와 빈 값), owner policy·packet·프롬프트(reader-check 추가,
  라운드 상향 문장을 `issueops model resolve --round`로 교체).
- 주입: `hostprotocol.RoleAgentArgs` 하나가 Claude `--agents` JSON과 Codex `-c agents.issueops-<role>.config_file=…`를
  만든다. Orca prepare·resume·reconcile은 `MarkInvoking`/`BeginIntent` 전에 계산해 terminal stage 요청에만 싣고,
  cmux는 `--` 앞에 붙인다. Orca Probe는 claude `--agents` 지원을 확인한다.
- 스킬: `skills/io-model` 추가, issueops-review·issueops-remote-write·issueops references 갱신.

## 성능 측정

`issueops next --id io-e66ba158272a --json`, base 빌드(`3a538328`)와 변경 빌드를 교차로 20회씩 실행(예열 3회).

| 측정 | base median | 변경 median | 차이 |
|---|---|---|---|
| 1차(설정 로드 2회) | 343.2 ms | 380.1 ms | +36.9 ms |
| 2차(호출당 로드 1회로 메모이즈) | 290.9 ms | 292.7 ms | +1.8 ms |

1차가 계획의 20 ms 기준을 넘었다. 원인은 `next`가 review를 두 번(초기값, 티어 반영) 해석하면서 매번
`git rev-parse`와 설정 파일 읽기를 반복한 것이다. `memoizedReviewModel`이 한 `next` 호출 안에서 저장소별로
한 번만 읽게 고친 뒤 차이는 +1.8 ms다.

## Side effect

- `model set --scope global`: `$XDG_CONFIG_HOME/issueops/agent-models.json`(없으면 `~/.config/issueops/…`) 한 파일.
- `model set|unset --scope local`: 메인 워크트리 `.issueops/agent-models.local.json`과 `<common-dir>/info/exclude` 한 줄.
- Orca·cmux 주입과 `model resolve --agents`(codex): state dir `agent-roles/<sha256>.toml`. 같은 내용이면 다시 쓰지 않는다.
- 추적 파일, `~/.claude/agents`, `~/.codex/agents`, `~/.codex/config.toml`은 바꾸지 않는다.
- record·intent 저장 형식은 그대로다. `IntentRequest.RoleAgentArgs`, `ExecutionOrcaIntentRequest.RoleAgentArgs`는
  `json:"-"`이고, `ReconcileIntentState.ProbeHost/ProbeRepo`는 메모리 안 상태다.
- 기본값 변경: Claude owner 기본 모델 `claude-sonnet-5-5` → `claude-opus-5-5`, Codex 리뷰 effort `xhigh` → `high`,
  Claude research·reader-check 기본값 추가.
- 실측 중 메인 저장소 `.git/info/exclude`에 `/.issueops/agent-models.local.json` 한 줄이 추가됐다(실측 뒤 local 파일은 지웠다).

## 게이트 원장 조정

- G11: PATH의 golangci-lint가 v1.64.8이라 v2 설정을 읽지 못했다. CI와 같은 v2.12.2를 임시 GOBIN에 설치해 실행하도록
  CHECK를 고쳤다(#550 원장과 같은 형태).
- G13: 처음 범위가 과거 이슈 기록·계획·ADR까지 포함해 이력을 고치지 않고는 통과할 수 없었다. 현행 표면으로 범위를
  좁히고, `IssueOpsPlannerDefaults` 같은 다른 이름에 걸리지 않도록 정확한 심볼 이름의 단어 단위 일치로 바꿨다.
- G12: 두 가지를 바로잡았다. (1) 이 머신의 python3에 pydantic·typer가 없어 Python 단계가 실패하므로 scratch venv를
  PATH 앞에 두고 원장을 실행한다. (2) 사용자 홈 설치본에는 새 스킬 `io-model`이 없어 native integration이 실패하고,
  `install-native.sh`는 연결 워크트리에서 실행해도 메인 체크아웃(stable root)을 설치한다. 그래서 worktree의 추적·미추적
  파일을 임시 독립 저장소로 복사하고, CI와 같은 임시 HOME 설치(`install-native.sh --skip-build --path-mode=skip
  --mcp-transport=stdio`) 뒤 self-verify를 실행하도록 CHECK를 바꿨다. 사용자 홈은 바꾸지 않는다.
- G16: `uv run --directory`가 작업 디렉터리를 바꿔 `--root .`이 스킬 폴더를 가리켰다. 계획대로 저장소 절대 경로
  (`os.getcwd()`)를 넘기도록 고쳤다.
- G13: DDD 인벤토리의 `source_symbol`은 정책이 원래 있던 위치를 남기는 이력 필드라 제외했다. 대상 심볼이 실제 소스와
  맞는지는 G9의 `TestDDDResponsibilityInventoryMatchesSource`가 검증한다.

## ai-slop-clean

측정: `scratchpad`의 Go AST 측정기(함수별 if·for·range·case·`&&`·`||` 수)와 golangci-lint v2.12.2 `dupl`.
범위는 변경된 비테스트 Go 파일 41개(추적 변경 + 미추적), base 버전과 같은 측정기로 비교했다.

| 지표 | 정리 전 | 정리 후 | 기준 |
|---|---|---|---|
| SNR(주석 줄 제외 비율, 근사) | 0.956 | 0.956 | ≥ 0.60 |
| 이번 변경이 새로 만든 분기 > 12 함수 | 2 (`runUnset` 17, `ValidateSetting` 13) | 0 | 0 |
| 이번 변경으로 분기가 늘어난 기존 > 12 함수 | `BuildInteractiveArgv` 16→19, `advanceOrca` 14→17, `Resume` 33→37 | 16→18, 14→15, 33→35 | 증가 최소화 |
| dupl(새 패키지) | 0 | 0 | 0 |
| import 비율 > 50% 파일 | 0 | 0 | 0 |

- needless-abstraction: 패키지 밖에서 쓰지 않는 export 6개(`Builtin`, `HostLayers`, `DocsOnlyReviewEffort`,
  `RoleAgentName`, `ExcludeLine`, `InjectedRoles`)를 unexport했다.
- 분기 축소: `modelWarnings`, `removeField`, `putLayer`, 세 서비스의 `terminalRoleAgentArgs`, `argsForStage`로 빼냈다.
  동작은 그대로이며 같은 focused 테스트로 확인했다.
- weak-artifact: 실제 이름과 다른 주석(`ConfigurableHosts`)을 고쳤다. `live-check.md`의 절대 경로에 사용자명이 들어가
  `meeting_notes_skill_contract_test`의 식별 데이터 검사에 걸렸으므로 `<source root>`·`<canonical worktree>`로 바꿨다.
- 남긴 것: `Probe`(41→43)·`Reconcile`(13→14)는 기존 함수에 nil 확인과 claude `--agents` 확인이 한 줄씩 늘어난 것이라 그대로 뒀다.

## 문서 반영

- 새 ADR `.issueops/adr/2026-10-08-role-models-resolve-from-global-and-local-settings-and-are-i.md`, ADR 색인에 등록하고
  2026-09-24 Claude 역할 모델 ADR을 부분 대체로 표시했다.
- 새 caution `.issueops/cautions/2026-10-08-claude-agents-self-verify-met.md`와 CAUTIONS 색인.
- `conventions/state-policy-and-hooks.md` §6, `architecture/runtime.md` 설정 위치 표, `architecture/issueops.md` 모델 문단,
  `architecture/host-integration.md` 스킬 수(53), `OPERATIONS.md` 스킬 목록, `operations/guides/issueops-execution.md` owner 기본값 문장.
- `adr/roadmap.md`: base부터 있던 줄 수 예산 위반(253 > 250)을 문구 변경 없이 두 문단의 줄바꿈만 합쳐 249줄로 맞췄다(G16).
- `README.md` 모델 표와 설정 안내, `README.en.md` 스킬 목록.
- `.issueops/*.md`는 project_docs MCP(read → revise SHA-CAS, append)로 고쳤다.

## 구현 리뷰

- 1라운드(독립 세션 `claude -p --model claude-opus-5-5 --effort high`, 빈 컨텍스트, 읽기 전용): revise, 차단 1건.
  owner 모델과 effort를 둘 다 명시한 Orca prepare `--confirm`에서는 설정을 읽지 않고 넘어가, 깨진 설정 파일이 Orca
  worktree 생성 뒤 owner packet의 `PolicyContext`에서야 드러나 intent가 unknown으로 남았다. 기존 테스트는 preview만 봤다.
- 수정: claude·codex는 플래그를 둘 다 명시해도 Probe·BeginIntent 전에 설정을 한 번 해석한다(값은 명시 플래그 우선,
  omo는 파일을 읽지 않으므로 그대로). `TestPreparationBrokenSettingsFailBeforeOrcaEvenWithExplicitOwnerFlags`가 preview와
  confirm 모두에서 trace가 load 직후 멈추는지 확인한다(수정 전 RED, 수정 후 GREEN).

## 실패와 재시도

- 첫 전체 테스트: 의도된 계약 변경(usage·response golden, DDD 인벤토리)만 실패했다. golden을 `-update`로 갱신하고
  diff가 `model` 명령과 `io-model` 스킬 추가뿐임을 확인했다.
- Claude print 모드에서 `--agents <json>` 뒤 위치 인자 프롬프트가 값으로 흡수되는 동작을 실측으로 확인했다(live-check.md).
