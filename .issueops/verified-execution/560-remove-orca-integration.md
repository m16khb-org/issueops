# #560 Orca 연동 제거: verified-execution report

- lifecycle: `io-7426b49dc042`, direct execution generation 2(generation 1 준비 세션이 release한 뒤 Herdr로 띄운 새 omp 세션이 `replace --preview`, `--reseed`, `claim --claim-current-token`으로 인수)
- issue: https://github.com/m16khb-org/issueops/issues/560
- branch: `560-remove-orca-integration`, base `main` @ `bac123aac302fd423673e60be0268c06696f9e13`(구현 진입 시 sync-base preview `merge_needed=false`)
- 계획: `.issueops/issues/560/plan.md`, 게이트 원장: `.issueops/issues/560/gates.md`
- 상태: 5단계 정리, 6단계 문서 반영을 끝냈고 7단계 검증 결과는 이 파일의 끝에 적는다.

## 수용 기준별 결과

| 기준 | 결과 | 근거 |
|---|---|---|
| 추적되는 Go 파일 어디에도 Orca 언급이 없다 | 충족 | G1 `NO_ORCA_GO`(`git grep --untracked -il orca -- '*.go'`, camelCase 경계의 `orCa` 부분 문자열 포함) |
| skills 아래 Orca 언급이 없고 자동 세션 선택이 Herdr 확인 뒤 현재 세션 또는 수동 터미널이다 | 충족 | G2 `NO_ORCA_SKILLS`, `TestIssueOpsAutomaticSessionSelectionOrderAndTakeoverChain` |
| usage golden과 `issueops --help`에 삭제한 명령과 방식 선택 플래그가 없다 | 충족 | G3 `USAGE_CLEAN`, 새 바이너리에서 `--mode`, `execution resume`, `execution switch-mode`, `doctor --preserve-terminal`이 모두 표준 오류로 거부됨(smoke) |
| 실행 기록 계약과 도메인에 방식 선택 식별자·필드가 남지 않는다 | 충족 | G4 `NO_MODE_FIELDS` |
| direct prepare, release, replace preview, reseed, claim 인계 체인이 끝까지 동작한다 | 충족 | G5, G10, 새 바이너리 임시 state smoke(아래) |
| CLI·MCP·response contract golden이 갱신된 표면과 일치한다 | 충족 | G6, `contract_golden`·`TestResponseContractsGolden` diff가 skill description 두 줄, 지운 필드, 해시뿐임 |
| 옛 모양 기록은 새 바이너리에서 읽히지 않는다(계획 결정 2) | 충족 | 실제 기록의 `mode`·`selection`·`driver`를 임시 state에 넣으면 `list --json`이 `read_errors: 1`을 보고함 |
| gofmt, vet, race 테스트, 빌드, self-verify 95점 게이트 | 충족 | G7, G7a, G8 |
| `.issueops` 문서 배치와 링크 계약에 위반이 없다 | 충족 | G11 `project docs check ok: 680 documents, 0 violations` |
| intent 문서의 성공 기준 "기존 direct 기록이 계속 읽힌다" | 사용자 지시로 대체 | 계획 단계에서 사용자가 "정리없이 코드만 제거"를 지시해, 성공 기준을 "옛 모양 기록은 새 바이너리에서 읽히지 않는 것을 영향으로 문서화하고 데이터 정리는 후속"으로 바꾸고 이슈 본문에 먼저 반영했다(feedback `contract_change`, 이슈 본문 갱신 기록 있음). |

## 구현 요약

- **계약**: `Execution.Mode`, `Selection`, `Orca`, `Workspace.Driver`와 타입 `ExecutionMode`, `ExecutionSelection`, `OrcaBinding`을 지웠다. 실행 API 요청·결과에서 `mode`, `resolved_mode`, owner 선택, issue snapshot, owner artifact 필드를 지웠다. `execution prepare`는 `--id ID ACTOR_FLAGS [--confirm]`만 받는다.
- **명령**: `execution resume`, `execution switch-mode`, MCP `resume` action, `doctor --preserve-terminal`, `doctor --sealed`와 `interactive` profile을 지웠다. stability-audit 스크립트와 테스트가 넘기던 `doctor --sealed`도 함께 지웠다.
- **lease·실행**: Orca intent 체인, reconcile의 Orca 경로(남은 것은 `remote_pr_create` 하나), 봉인 claim context, owner prompt·context packet, replacement의 Orca owner inventory·runtime rollover를 지웠다. generation·actor·cwd fence, claim 토큰 검증, workspace 프로세스 정지 확인은 그대로다. `next`는 claimable이면 `claim --claim-current-token`, released면 `replace --preview`, revoking이면 `replace --finalize-preview`를 안내한다.
- **정리·진단**: cleanup finish의 `orca_remove`, abandon의 `orca_resources_absent`, Orca 터미널 처리, doctor의 Orca 수집기와 분류 어휘를 지웠다. 사이클 생존 판정은 holder 식별자로만 한다.
- **인계 관측**: launcher는 `direct`, `herdr`, `cmux`만 남는다. manual-direct 관측(source generation N과 claim N+1 연결)은 유지한다.
- **어댑터**: `internal/adapter/orca`, `internal/port/orca*`, Orca provisioner wrapper를 지웠다. 조립은 `cmd/issueops/issueopsapp`에서만 바꿨다.
- **orphan**: 위 삭제로 호출자가 사라진 함수를 `deadcode ./cmd/...`로 찾아 지웠다(owner 정책용 명령 문자열 파서 `ParseExactIssueOpsCommand`와 `shelltoken`의 활성 확장 검사 함수군, GitLab issue snapshot 검증, `RequireMutableLeaseSnapshot` 등). 기준선은 0줄이었고 지금도 0줄이다.
- **이름 변경**: 대소문자 무시 `orca` 검사에 걸리던 camelCase 이름(`prepareInstallPathPlanForCandidate` 등 18개 파일)을 의미 변경 없이 바꿨다.
- **스킬·문서**: 스킬 14개와 현행 `.issueops` 문서, README 두 개를 새 계약으로 고쳤고 새 ADR `2026-10-15-remove-orca-integration-and-execution-mode-selection`을 추가했다.

## 실제 바이너리 smoke

worktree에서 빌드한 `./bin/issueops`를 임시 `ISSUEOPS_STATE_DIR`과 임시 Git 저장소에서만 실행했다(실제 user state와 source checkout의 `bin/issueops`는 건드리지 않음).

| 단계 | lease(generation, status) |
|---|---|
| `execution prepare` preview, `--confirm` | 1 active |
| `execution release` | 1 released |
| `execution replace --preview` | 1 released, inventory fingerprint 발급 |
| `execution replace --reseed --confirm` | 2 claimable, next_command가 `claim --claim-current-token` |
| `execution claim --claim-current-token` | 2 active, holder가 claim actor |

마지막에 같은 state의 `list --json`은 `read_errors: 0`이었다.

## side effect 목록

- 저장소: Go·스킬·문서·golden 파일 삭제와 수정, 새 ADR 한 건, 새 테스트 파일(인계 체인 통합 테스트, claim 관측 테스트, 플래그 거부 테스트).
- durable state: 이 사이클의 기록은 옛 모양이므로 merge 전까지 PATH의 옛 `issueops`로만 lifecycle 명령을 실행했다. 새 바이너리가 쓴 기록은 옛 바이너리가 읽지 못한다.
- 읽히지 않는 로컬 데이터(정리하지 않음, 후속 작업): 옛 바이너리가 쓴 실행 기록 18개와 orca launcher 줄이 있는 인계 감사 파일. 옛 모양 실행 기록이 state에 하나라도 남아 있으면 새 바이너리에서는 그 사이클뿐 아니라 다른 모든 사이클의 `execution prepare`도 `invalid state`로 실패하고(소유권 스캔이 모든 기록을 읽는다), 이 기록을 치우는 명령은 이번 범위에 없다. 그래서 업데이트 전에 옛 바이너리로 기존 사이클을 `cleanup finish`나 `cleanup abandon`으로 끝내야 한다(계획 결정 3). 감사 파일 정리 전까지 이 기기에서 `execution handoff-cmux`와 manual claim 관측은 실패한다.
- fingerprint: replacement inventory, quiescence, cleanup inventory fingerprint의 입력이 바뀐다. 업데이트 직후 진행 중인 preview는 다시 받아야 한다.
- 외부: 사용자 머신의 Orca 리소스, 저장소 밖 전역 스킬은 건드리지 않았다. 저장소 밖 호출자가 지운 플래그나 명령을 쓰면 업데이트 직후 실패한다(ADR에 적음).
- 검증 환경: `skills/slack-delegate`의 Python 테스트가 `pydantic`을 요구해, G8 실행 때 임시 venv(`/tmp/gvenv`)를 PATH 앞에 두었다. 저장소 파일은 바뀌지 않는다.

## 성능

hot path가 아니다. 측정값은 아래와 같다.

| 항목 | 이전 | 이후 |
|---|---|---|
| `bin/issueops` 크기 | 33,358,658 B | 31,765,618 B (-1,593,040 B, -4.8%) |
| 변경 파일 수 / 삭제 줄 | | 693개 / 55,165줄 삭제, 3,142줄 추가(Go 605개 파일, 50,092줄 삭제, 2,508줄 추가) |
| `go test -race ./...` | | 약 250초(G7 한 번의 전체 통과) |

외부 프로세스 호출은 Orca RPC와 Orca inventory 조회가 사라진 만큼 줄었다.

## 문서 → 구현 대조

| 문서 경로 | 규칙 문장 인용 | 지킴 또는 어김 | diff의 근거 위치 |
|---|---|---|---|
| `.issueops/CONSTITUTION.md` | "핵심 정책은 host adapter가 아니라 Go core에 둔다." | 지킴 | Orca 제공 부분을 core·port·application에서 지웠고 host별 어댑터에는 동작을 더하지 않았다. 스킬은 문구만 바꿨다. |
| `.issueops/CONSTITUTION.md` | "CLI/MCP contract 변경은 JSON schema/golden test와 실제 command smoke test를 남긴다." | 지킴 | `cmd/issueops/testdata/{usage.golden.txt,mcp_tools.golden.json,response_contracts.golden.json}`, `internal/adapter/testdata/native_install_contract_matrix.golden.json`, 위 smoke 표 |
| `.issueops/CONVENTIONS.md` | "`internal/adapter/*`는 composition root(`cmd/issueops/issueopsapp`)에서만 조립된다. 금지 adapter edge는 0이다." | 지킴 | `internal/adapter/orca` import는 `cmd/issueops/issueopsapp`에서만 지웠고 다른 계층에 새 의존을 만들지 않았다. `internal/architecture` 테스트 통과 |
| `.issueops/ARCHITECTURE.md` | "IssueOps v1 execution은 단일 authority다. generation/actor/cwd fence는 core가 소유한다." | 지킴 | claim·release·replace·reseed의 fence 코드는 Orca 분기만 지웠다. `TestDirectHandoffChainThroughProductionCommands`가 generation 순서와 holder를 단언한다. |
| `.issueops/CAUTIONS.md` | "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." | 지킴 | 문서 수정이 끝난 뒤 `-update`로 golden을 재생성했고 diff가 skill description, 지운 필드, 해시뿐임을 확인했다. |
| `.issueops/CAUTIONS.md` | "게이트 원장 `CHECK:`는 argv 한 줄이다." | 지킴 | 게이트 13개를 모두 `python3 -c` 한 줄이나 `go test` 한 줄로 썼고 `gates lint`가 문제 0건이었다. |

이 diff가 어긴 규칙은 없다.

## 적용한 문서 규칙

| 문서 경로 | 해당 문장 인용 | 이 구현에서 바뀐 결정 |
|---|---|---|
| `.issueops/CONSTITUTION.md` | "CLI/MCP contract 변경은 JSON schema/golden test와 실제 command smoke test를 남긴다." | usage·MCP·response golden을 재생성하고 diff를 검수했다. 새 바이너리로 인계 체인 smoke를 실행했다. |
| `.issueops/ARCHITECTURE.md` | "IssueOps v1 execution은 단일 authority다. generation/actor/cwd fence는 core가 소유한다." | Orca 제공 부분과 방식 선택만 지웠고 fence는 약화하지 않았다. |
| `.issueops/AGENT_WORKFLOW.md` | "브랜치와 worktree를 준비한 뒤 Herdr를 확인한다." | 자동 세션 선택 순서를 Herdr 확인 뒤 현재 세션 또는 수동 두 번째 터미널로 두었다. |
| `.issueops/CAUTIONS.md` | "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." | 문서 수정이 끝난 뒤 golden을 재생성했다. |
| `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md` | "함수 단위는 golang.org/x/tools/cmd/deadcode ./cmd/...로, 타입·alias는 production 참조 0 스캔으로 확인한다." | 삭제 뒤 `deadcode`로 새 orphan 함수를 찾아 지웠다. 테스트는 새 모양으로 고치거나 지웠다. |
| `.issueops/TESTING.md` | "완료 보고의 evidence는 단일 "전 단계 통과" run에서 나온 것이어야 하며" | 마지막 변경 뒤 게이트 원장 전체를 한 번 더 실행한다(7단계). |

## ai-slop-clean

- 제거한 것: 위 `orphan` 항목의 호출자 없는 함수와 그 전용 테스트, 사용처가 없어진 `CleanupProcessDeps.Getenv`, `resolveAgentModel`, 지운 명령을 설명하던 주석.
- 의도적으로 유지한 것: `execution handoff-cmux`와 cmux 코드, `ExecutionIssueSnapshot` 계약(cleanup finish가 쓴다), `AccessResult`(direct의 base 접근 검사가 쓴다), `domain/issueopsartifact.CanStage`의 미사용 `name` 인자(호출자가 여럿이라 이번 범위에서 시그니처를 바꾸지 않음).
- 범위 밖 발견: 없음.

## 검증 (7단계)

게이트 원장 `.issueops/issues/560/gates.md`의 EVIDENCE가 마지막 전체 실행의 결과다. 구현 리뷰 지적을 고치려고 구현 단계로 돌아온 뒤의 재실행이라서 `gates check --write`로 원장에 기록했다. 원장은 이 사이클의 변경이 모두 끝난 뒤 `./bin/issueops`를 다시 빌드하고 한 번의 `gates check` 실행으로 13개 게이트를 처음부터 모두 돌렸다. G8의 self-verify가 `skills/slack-delegate`의 Python 테스트 때문에 `pydantic`을 요구하고 native integration이 `~/.omo`를 요구해, 임시 venv와 이전 `~/.omo`의 읽기 전용 심볼릭 링크를 담은 임시 HOME에서 실행했다. 두 환경은 저장소 파일을 바꾸지 않는다.

| 요구한 문서 경로 | 그 문서의 해당 문장 인용 | 명령 | 결과 |
|---|---|---|---|
| `.issueops/TESTING.md` | "Go 코드 변경의 기본 검증(`gofmt -l`, `go test -race ./...`, `go vet ./...`" | `gofmt -l` | 출력 없음 |
| `.issueops/TESTING.md` | 같은 문장 | `go vet ./...` (G7a) | `VET_PASS` |
| `.issueops/TESTING.md` | 같은 문장 | `go test -race ./... -count=1` (G7) | `RACE_PASS` |
| `.issueops/TESTING.md` | "검증 명령을 실행할 최신 binary가 없으면 먼저 빌드한다." | `go build -o bin/issueops ./cmd/issueops` | 통과 |
| `.issueops/TESTING.md` | "단일 self-verify의 완전한 성공 결과로 test/build/golden/docs/inspect 기본 게이트를 확인한다." | `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json` (G8) | `SELF_VERIFY_OK` |
| `.issueops/CAUTIONS.md` | "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." | `go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1` (G6) | `ok` 두 패키지 |
| `.issueops/testing/unit-and-contract.md` | "`GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...`." | golangci-lint v2.12.2를 `GOTOOLCHAIN=go1.26.6`, 새 캐시로 darwin과 `GOOS=linux`에서 실행하고 base `bac123aa`와 같은 방법으로 비교 | base 90건(서로 다른 파일·메시지 쌍 17개), 변경 후 19건(7개), 중복 횟수까지 센 비교로 새 지적 0건. 변경 후의 지적은 모두 base에 있던 것이다. base의 CI Lint 단계는 이미 실패하는 상태다. 이 변경이 새로 만들 뻔한 테스트 helper `stageIssueOpsArtifact`의 unused 지적은 파일을 지워 없앴다. |
| `.issueops/cautions/2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do.md` | "함수 단위는 golang.org/x/tools/cmd/deadcode ./cmd/...로" | `go run golang.org/x/tools/cmd/deadcode@latest ./cmd/...` | 출력 0줄(기준선 0줄) |
| 게이트 원장 | G1~G12와 G7a | `issueops gates check --file .issueops/issues/560/gates.md --write` | 13개 모두 met |
| 스킬 | `issueops-verify` 4절 | `issueops execution sync-base --preview` | `merge_needed: false`, 충돌 없음, base `bac123aa`와 작업 브랜치가 같다 |

- 병렬 실행 이득: 구현 리뷰 서브에이전트를 게이트 실행과 동시에 돌리지 않았다. 리뷰 1라운드 지적이 문서와 코드 정리로 이어져 게이트를 한 번 더 돌려야 했고, 그래서 `parallel_speed`로 절약한 시간은 없다.
- 구현 리뷰: 1라운드 `revise`(필수 결함 3건 해소), 2라운드 `revise`(필수 결함 1건 해소), 3라운드 `revise`(필수 결함 1건 해소), 4라운드 `pass`. 이후 report 문구만 고쳤고(lint 건수 표기, 리뷰 이력) 5라운드 delta 리뷰로 그 문구 변경만 다시 확인했다. 마지막 판정은 `issueops status`의 `implementation_review`에 기록한다.
- self-verify 범위: base와 HEAD가 같은 커밋(`bac123aa`)이라 `--base-ref`를 주든 안 주든 검사 범위가 같다.
- Not Run: `issueops api-doc check`(endpoint·DTO·OpenAPI 변경이 없고, 바뀐 것은 CLI·MCP 계약뿐이라 contract golden과 conformance baseline 테스트가 대신한다), 화면 QA(`review.frontend` false), 스키마 실측(변경 집합에 마이그레이션·SQL 파일 없음).
