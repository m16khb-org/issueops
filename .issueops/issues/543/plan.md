# 커밋된 변경을 포함한 risk QA 검증 계획

Lifecycle: io-ca6dfd9d657f
Issue: https://github.com/m16khb-org/issueops/issues/543
Source: $SOURCE_ROOT
Branch: 543-self-verify-committed-risk-scope
Prepared base: main @ f8e7c37fd9ff09d68536aee244cf560560e4d919
Implementer: gpt-6.1-sol / high. direct canonical worktree를 준비한 후 root coordinator가 감독하는 Orca worker에 같은 worktree를 인계한다. 구현·검증·커밋·푸시·Draft PR·execution complete까지 worker가 수행하고, 사용자가 승인한 머지·cleanup·io-update는 coordinator가 수행한다. 워커는 다른 사이클과 source checkout을 변경하지 않는다.

## 문제와 수락 기준

현재 risk_qa_git.go:11의 gitChangedPaths는 git status만 수집한다. application/riskqa/plan.go:14는 경로를 기존 domain 분류기에 넘기고, execute.go:27은 명령이 없으면 성공을 반환한다. 따라서 clean committed Go diff를 지정 기준으로 검증할 방법이 없다.

1. 선택 기준 커밋을 받으면 그 커밋의 tree부터 한 번 고정한 현재 HEAD tree까지의 tracked diff, staged/unstaged diff와 untracked 경로를 합쳐 기존 분류기에 전달한다. merge-base 의미로 바꾸지 않는다. commit-only Go 변경은 기존 분류 규칙의 vet/race를 선택한다.
2. 기준·HEAD는 full commit OID로 해소하고 risk plan의 범위 근거에 남긴다. 현재 Execute가 stdout 앞의 plan JSON을 8 KiB tail 절삭으로 잃을 수 있으므로, 범위 근거와 scope failure를 출력 절삭과 독립적으로 보존한다. 최소 방식은 bounded 검사 stdout tail 뒤에 작은 scope JSON 근거를 결합해 성공과 실패 StepResult 모두에서 살아 있도록 하는 것이다. generic StepResult/온디스크 schema의 확장은 필요하지 않으면 하지 않는다. 성공·실패 로그가 각각 8 KiB를 넘어도 최종 CLI/MCP result에서 기준·HEAD와 scope failure를 확인하는 회귀 테스트를 추가한다. 기존 aggregate budget과 실패 진단은 유지한다. 잘못된 ref, Git 관측 실패, range 실패는 성공이나 정상 no-op으로 처리하지 않는다. 유효한 빈 범위는 정상 no-op이다.
3. CLI `self-verify --base-ref REF`와 MCP `self_verify`의 `base_ref`가 같은 LoopRequest와 scope로 전달된다. ref를 지정하지 않은 standalone 호출은 현행 working-tree 동작을 유지한다.
4. 이 저장소의 IssueOps verify 및 self-verify 스킬은 current exact cycle status의 `branch_prepare.base_sha`를 읽어 기준으로 넘긴다. clean committed 사이클 검증에서도 범위를 전달하며, 선택 API를 만들고 호출하지 않는 상태로 끝내지 않는다. 여러 사이클이 있어도 임의로 선택하지 않는다.
5. 공백·quote·newline·rename 경로를 기계 판독 가능한 -z 형식으로 수집한다. NormalizePaths(paths.go:13)의 TrimSpace는 실제 Git 파일명의 양끝 공백/newline을 삭제하므로 collector부터 classifier까지 경로 bytes를 보존하도록 최소 수정한다. 비어 있는 경로만 제외하고 정확한 경로 기준으로 중복을 제거한다. 양끝 공백/newline과 공백 유무만 다른 두 파일이 최종 changed_paths에 구별되어 남는 회귀 테스트를 추가한다. rename source와 destination을 모두 포함해 Go 파일이 다른 확장자로 rename될 때도 Go 영향이 사라지지 않도록 한다. external diff/textconv 실행을 비활성화한다.
6. CI 독립 vet/race와 기존 실패 출력, full-suite coverage reuse는 유지한다. scope 미관측 실패는 coverage true로 표기하지 않는다.

비목표: 자동 origin/main 추론, active cycle 전역 자동 선택, ref fetch, Git 변경/remote 변경, diff 텍스트 분석, 새 캐시, CI 검사 정책 변경, self-verify 단계 재설계, 품질 감사표 수정과 검증 문서의 중복 지침 정리.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장·제3장: 명령은 명시 cwd·timeout을 사용하고 Git 관측 실패를 정상 검증으로 승격하지 않는다. core 규칙은 host adapter에 복제하지 않는다.
- `.issueops/ARCHITECTURE.md` 의존 방향 / `architecture/hexagonal-core.md`: Git 프로세스 관측은 adapter, 순수 경로 분류는 domain, 실행 계획 조합은 application에 유지한다. root만 concrete adapter를 조립한다.
- `.issueops/CONVENTIONS.md` CLI/MCP / 산출물: snake_case 필드, 같은 공용 요청 의미, `.issueops/issues/543/` 산출물과 ignored artifact를 사용한다.
- `.issueops/CAUTIONS.md` universal summary / `cautions/audit-and-process.md` 자기 검증 drift·named RED: 신규 실패를 먼저 재현하고 same revision의 command/result를 보존한다. 실패를 성공으로 덮지 않는다.
- `.issueops/ADR.md` accepted baseline: 외부 Go core + 얇은 host adapter, standalone 검증을 유지한다. 자동 외부 도구 설치나 readiness 추가가 없다.
- `.issueops/TESTING.md`, `testing/unit-and-contract.md`: same bundle에서 실제 실행한 vet/race 근거와 full golden을 확인한다. `testing/self-verification.md`의 현행 단일 pass 계약을 따르되 이 파일은 병렬 docs 사이클이 소유하므로 수정하지 않는다.
- `skills/issueops-verify/SKILL.md` §1: 성공 증거는 exact input/fingerprint/environment에 묶어 재사용한다. this repo self-verify 호출에 prepared base를 전달하는 지침만 추가한다.

## 재사용하는 기존 구현

- `internal/domain/riskqa/plan.go`의 PlanFromPaths와 CoversFullGoTest는 경로 입력·명령 선택의 SSOT로 유지한다. 새 범위 분류기를 만들지 않는다.
- `internal/contract/riskqa/plan.go` RiskQATierPlan에 범위 근거와 실패 상태를 최소 additive 필드로 표현한다. 현재 필수 tier/changed_paths/reasons/commands를 제거하지 않는다.
- `internal/application/riskqa/plan.go` Service의 ChangedPaths dependency injection, `execute.go` ExecuteDeps를 기존 구조 안에서 확장한다. 선택 scope는 입력 값으로 전달하며 ambient env/global cache로 숨기지 않는다.
- `internal/adapter/verification/riskqa/risk_qa_wrapper_test.go`의 임시 Git helper와 injected runner를 재사용한다.
- `cmd/issueops/selfworkflow/verifycmd/verify.go` → `internal/application/selfverify/loop.go` LoopRequest → `cmd/issueops/issueopsapp/self_verify_facade.go` newSelfWorkflowExecutor의 root wiring을 통해 기준 scope를 적용한다.
- `cmd/issueops/mcpcli/mcp_tool_self.go:36`, `internal/contract/mcp/tool_schemas.go:146`의 self_verify만 같은 선택 필드를 추가한다. full catalog 생성/golden helper를 사용한다.

## 성능 영향

self-verify 한 pass의 cold-path에 scope 지정 시 기준/HEAD commit 해소와 bounded Git diff/status 관측을 추가한다. 전체 diff 본문 대신 경로 목록만 읽으며 시간·메모리는 경로 수에 비례한다. HEAD OID와 plan은 한 번 관측해 명령 실행과 coverage 재사용에서 공유한다. 전역 캐시나 요청 간 mutable singleton은 추가하지 않는다. scope 미지정 경로에는 필요 없는 commit 관측을 추가하지 않는다. focused tests의 wall time과 scope 관측 횟수를 기록하며 측정 없이 성능 개선을 주장하지 않는다.

## 하위 호환성과 side effect

CLI 플래그와 MCP optional schema 필드는 additive이며 기존 무기준 호출은 같은 working-tree 입력을 유지한다. 기존 JSON 필수 필드를 유지하고 새 범위·오류 근거를 같은 CLI/MCP plan 출력에 담는다. strict unknown-field 또는 golden 소비자는 catalog와 fixture를 함께 갱신한다. request의 기준 입력은 shell 문자열이 아닌 argv로 전달하고 commit OID로 해소한다. 이 값으로 Git/파일/원격을 수정하지 않는다. record schema·IssueOps lifecycle 모델·설치 경로는 바꾸지 않는다. 롤백은 해당 PR을 revert하는 코드 롤백이다.

스킬은 agent가 읽는 절차 산문만 최소 수정한다. 리뷰어나 LLM prompt payload를 바꾸지 않는다. `skills/issueops-verify/SKILL.md`의 this repo self-verify 호출과 `skills/self-verify/SKILL.md`의 명령 예시·범위 규칙에 기준 전달을 명시하고, 누락 base는 무기준 실행으로 우회하지 않도록 한다. `.issueops/testing/unit-and-contract.md`의 working-tree-only 문구만 실제 scope 계약으로 바꾼다. 다른 docs 사이클 소유 파일 `.issueops/TESTING.md`, `.issueops/testing/self-verification.md`를 수정하지 않는다.

## 실행 단계와 검증

### 1. RED와 최소 공용 범위 확장

Owner gpt-6.1-sol/high. canonical path·branch·HEAD·source status·lease를 매 편집 배치에 확인한다. allowed ownership은 riskqa contract/application/domain/adapter, selfverify LoopRequest 전달과 facade, verifycmd CLI, mcpcli dispatch와 contract/mcp self_verify schema, 필요한 catalog/golden, 위 세 문서/스킬, `.issueops/issues/543/`이다. 다른 worker 변경은 되돌리지 않는다.

임시 Git repo에 base commit 뒤 Go 파일 commit과 clean status를 만들고 PlanWithScope 또는 선택 입력 결과가 vet/race를 선택하는 named regression을 먼저 FAIL로 기록한다. production 변경 전 exact test의 RUN/FAIL과 exit 1을 보존한다. 이후 기존 collector와 분류기를 확장해 GREEN으로 만든다. staged/unstaged/untracked union, 유효한 empty range, invalid/missing base, non-repo, quoted/space/newline/rename Go→non-Go 경로, 양끝 whitespace 구별, 성공·실패 oversized output에서 scope evidence 보존에 focused cases를 추가한다. 관측 실패가 risk step 실패와 coverage false로 전파되는 application/adapter case도 포함한다.

### 2. CLI/MCP와 실제 호출 연결

CLI request capture와 MCP request capture가 같은 base_ref를 전달하는 named test를 추가한다. facade/loop의 production wiring을 거쳐 scope가 risk plan에 도달함을 focused integration에서 확인한다. prepared base 지침과 self-verify 명령 예시를 위 소유 파일에 적용하고 스킬 validator·shell validator와 scoped spec 검사로 base 전달 지침 누락을 막는다. MCP DTO/schema가 바뀌므로 OPEN_API_SPEC gate 적용 필요 여부를 현재 문서/도구로 확인하고 적용 대상이면 정적 및 agent gate를 실행한다.

### 3. SURFACE→CLEAN→문서 판정→검증·리뷰→PR

게이트 원장은 `.issueops/issues/543/gates.md`에 init하고 실제 CHECK로 EVIDENCE를 채운다. focused GREEN 후 diff를 정리하고 full deterministic battery를 실행한다. risk scope에 봉인 base를 전달해 vet/race가 실제로 실행됐는지 단일 결과에서 확인한다. full suite가 race에 포함돼 재사용되면 일반 Go test를 중복 실행하지 않는다. 실패/코드 수정 시 이전 부분 결과를 합치지 않고 새 single pass를 실행한다. 수정 뒤 slop/docs gate를 현재 fingerprint에 기록하며 독립 fresh-context diff review를 통과한 뒤 커밋·푸시·Draft PR과 execution complete까지 진행한다. worker는 머지·cleanup·update를 실행하지 않는다.

## 게이트 명세

- G1: 기준 범위의 commit-only Go, dirty union, invalid scope가 정확히 판정된다 | CHECK: go test ./internal/adapter/verification/riskqa ./internal/application/riskqa ./internal/domain/riskqa -count=1 | EXPECT: /^ok\s/m
- G2: CLI·MCP 기준 전달과 selfverify 연결이 동일하게 동작한다 | CHECK: go test ./cmd/issueops/selfworkflow/verifycmd ./cmd/issueops/mcpcli ./internal/application/selfverify ./cmd/issueops/issueopsapp -count=1 | EXPECT: /^ok\s/m
- G3: issueops-verify 스킬 형식이 유효하다 | CHECK: python3 scripts/validate-skill.py skills/issueops-verify | EXPECT: /Skill is valid/
- G4: self-verify 스킬 형식이 유효하다 | CHECK: python3 scripts/validate-skill.py skills/self-verify | EXPECT: /Skill is valid/

Full battery evidence는 원장 중복 실행 대신 repository completion gate 결과를 report에 둔다. 빌드한 local binary에서 `self-verify --base-ref <prepared-base-sha> --seed=100 --target-score=95 --llm-eval=false --json`의 .ok/.termination_eligible/.summary.termination_eligible와 risk scope base/head, vet/race command를 검증한다. startup 설치 권한은 사용자 update 요청 범위 안이지만 이번 구현 worker는 설치를 실행하지 않는다.

## 인계와 중단 규칙

Root coordinator는 계획 검토 pass, direct prepare의 exact fingerprint confirm, canonical branch/HEAD, generation과 released state를 확인한 후 같은 worktree에서 Orca worker-start를 한 번 실행한다. 수신 worker는 status/next와 sealed plan/intent/digest를 먼저 대조하고 자기 native whoami flags로 claim 체인을 따른다. 새 자동 인계를 재실행하지 않는다. 다른 live holder, stale review, branch/root/generation 불일치, 원격 생성 ambiguity, 필수 검증 실패는 우회하지 않는다. 복구 가능한 결함은 조사·수정·재검증하고, 불가하면 concrete blocker와 결과 경로를 coordinator에 남긴다.
