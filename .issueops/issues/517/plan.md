# 리뷰 3라운드에서 Fable 5를 자동 선택하지 않게 한다: 구현 계획 (3차)

- lifecycle ID: `io-312a238fdcfe`
- 이슈: https://github.com/m16khb-org/issueops/issues/517
- 브랜치: `517-review-round3-no-fable` (base `main` @ `92bbbddabb9bbee9c7a1e050fc6fe061d7d45201`)
- 사용자 요청 범위: 2026-09-24 "fable은 명시적으로 사용하지않으면 사용하지않아야하는데 왜사용하는거지? 코드베이스에 그렇게 되어있다면 고쳐". 전체 사이클이며 종료점은 draft PR 발행과 `issueops execution complete`다. merge와 post-merge 정리는 범위 밖이다.
- 세션 인계: 계획 단계 끝에서 `execution prepare --mode direct --owner-model claude-opus-5-5 --owner-effort high`로 워크트리를 준비하고, Orca가 ready이면 같은 워크트리에 `claude --model claude-opus-5-5 --effort high` 새 세션을 띄워 인계한다(사용자 지시: 구현은 Opus 5.5 high). 인계는 요청 범위를 넓히지 않는다.
- 병행 사이클: #513(PR #516)도 `skills/issueops-review/SKILL.md`(128행 근처, 177행 근처), `.issueops/adr/README.md`·`.issueops/ADR.md`(새 ADR)를 바꾼다. 이 계획은 스킬의 3라운드 규칙 문단(144~147행)과 "나쁜 예" 목록, ADR 색인과 README 대체 목록의 새 항목만 더한다. 병합 때 나중 쪽이 rebase한다.

## TL;DR

> **요약**: `issueops-review` 스킬의 3라운드 규칙에 Claude 전용 하위 항목 두 개를 추가한다. "Fable 5는 3라운드용으로 고르지 않는다"와 "Claude는 같은 모델을 쓰고 effort를 `next.review.effort`에서 한 단계 올려 `claude -p`로 띄운다"이다. 2026-07-24 ADR에 남은 옛 Claude 역할 모델 기본값(fable5/opus4.8)은 새 ADR로 대체하고 README 대체 목록과 ADR 색인에 올린다.
> **산출물**: 스킬 문단 수정, 새 ADR 1개, `.issueops/adr/README.md`·`.issueops/ADR.md` 갱신
> **규모**: Quick
> **병렬**: NO
> **Critical Path**: T1 → T2 → 검증 → 커밋·게시

## Context

### Original Request

사용자는 에이전트가 계획 리뷰 3라운드에 Fable을 쓴 것을 지적하고, 코드베이스가 그렇게 되어 있다면 고치라고 했다. 원문은 record의 `intent.raw_request`에 있다.

### Interview Summary

- 추가 질문은 없었다. 코드와 운영 문서는 이미 Fable 5를 명시적 수동 지정 전용으로 정했다. 규칙과 어긋난 곳은 리뷰 스킬의 3라운드 문구와 옛 ADR의 기본값 표기다.
- codex와 omo의 3라운드 모델 선택은 바꾸지 않는다(intent 비목표).

### 1차 계획 리뷰 반영 (판정 revise, 지적 R1-1~R1-5)

- R1-1: 스킬 문구와 G3에서 effort 값 하드코딩을 없애고 `next.review.effort` 기준으로 적었다(Gap Analysis 2).
- R1-2: golden이 바뀌지 않는다는 실측에 맞춰 Deliverables, T2, QA, 커밋 목록에서 golden을 뺐다(Gap Analysis 4).
- R1-3: 전체 테스트 게이트의 시간을 실측해 정했다(Verification Strategy).
- R1-4: G7이 ADR 색인 링크의 실제 파일명까지 확인한다.
- R1-5: 첫 하위 항목을 Claude로 한정하고 codex·omo는 적용 대상이 아니라고 적었다. G3이 이 문장을 확인한다.

### 2차 계획 리뷰 반영 (판정 revise, 지적 R2-1)

- R2-1: G13(self-verify)의 시간과 실패 처리를 정했다. 이 사이클은 Go 파일을 바꾸지 않아 self-verify가 race를 돌리지 않는다는 코드 근거, 로컬 819초 실패의 원인, 같은 날 CI 통과 기록, 선실행과 재실행 규칙을 Verification Strategy에 적었다.

### Gap Analysis

1. **Claude의 effort 지정 수단**: Claude의 서브에이전트 도구는 모델만 고를 수 있다. 그래서 3라운드 effort 상향은 `claude -p --model <model> --effort <effort>` 새 세션으로 한다. 2026-09-24 #514 3라운드에서 이 방법으로 `claude-opus-5` xhigh 리뷰를 실행했고, 판정이 record에 남았다(`io-34938e479083` devils_advocate_review, 04:36:10Z).
2. **effort 단계**: claude CLI의 `--effort`는 `low`, `medium`, `high`, `xhigh`, `max`를 받는다(`claude --help`). `next.review.effort`는 변경 tier에 따라 달라진다. Claude 기본은 `high`이고 docs-only 변경은 `medium`이다(`internal/port/orca.go:25-40`의 `IssueOpsReviewEffortForTier`). 스킬에는 값을 복사하지 않고 "`next.review.effort`에서 한 단계 위"로 적는다. 스킬 34-35행이 모델·값 표를 본문에 복사하지 말라고 정했기 때문이다(1차 계획 리뷰 R1-1).
3. **ADR 규칙**: ADR은 추가만 한다. 옛 기록은 고치지 않고, 새 날짜의 기록이 무엇을 대체하는지 밝히며 README 대체 목록에 올린다(`.issueops/adr/README.md` Status model, Naming and authoring). 새 ADR은 `project_docs_append(kind=adr)` 또는 `issueops project append --kind adr`로 만들고, ADR 색인과 README는 SHA 확인을 거치는 `project_docs_revise`로 고친다.
4. **golden**: 현재 `docs_index` 투영은 `docs_count`를 정규화하고 필수 문서 존재 여부만 담는다(`cmd/issueops/issueopsapp/response_contract_docs_projection_helper_test.go:23-47`, `response_contract_golden_test.go:118,176`). 그래서 이 사이클의 문서 추가로 golden은 바뀌지 않는다. §27이 말하는 bytes·headings·title 캡처는 현재 투영과 맞지 않지만, §27 문구 정정은 이 이슈 범위 밖이다(1차 계획 리뷰 R1-2).
6. **게이트 시간 제한**: 명령 정책은 CHECK 시간 제한을 15분까지만 허용한다(`internal/adapter/policy/policy_evaluate.go:74`). 전체 테스트 게이트는 실측 시간이 900초 안에 여유 있게 들어오는지 확인해 정한다(1차 계획 리뷰 R1-3, 아래 Verification Strategy).
7. **host 범위**: 새 하위 항목은 Claude에만 적용한다. codex와 omo의 3라운드 규칙은 그대로다(1차 계획 리뷰 R1-5).
5. **이미 기록된 판정**: #513의 계획 리뷰 3라운드 pass(fable)와 #514의 중단된 fable 리뷰는 기록을 고치지 않는다(intent 비목표).

## 적용되는 결정과 주의사항

| 문서 | 항목 | 이 계획에 주는 제약 |
|---|---|---|
| `internal/contract/issueopspreparation/prepare.go:18-19`, `.issueops/architecture/issueops.md:42`, `.issueops/operations/guides/issueops-execution.md:183`, `skills/issueops/references/execution.md:386-388` | Fable 5는 명시적 수동 지정 전용 | 스킬 문구가 이 규칙을 그대로 따르게 한다. 코드의 기본값은 바꾸지 않는다. |
| `.issueops/adr/README.md` | ADR은 추가만 하고 대체는 새 기록이 밝힌다 | 2026-07-24 기록은 고치지 않는다(G6). 새 ADR과 README 대체 목록으로 처리한다. |
| `.issueops/cautions/issueops-lifecycle.md` §27 | `.issueops/*.md` 편집은 golden을 어긋나게 한다 | 현재 투영에서는 문서 추가로 golden이 바뀌지 않는다(Gap Analysis 4). T2에서 `-update`로 재생성해 diff가 비어 있는지만 확인한다. |
| `internal/adapter/policy/policy_evaluate.go:74` | CHECK 시간 제한 상한 15분 | 게이트의 `--timeout-seconds`는 900을 넘길 수 없다. 전체 테스트 게이트는 실측으로 정한다. |
| `.issueops/cautions/2026-09-23-gates-init-spec-pipe-segment.md` | 게이트 spec의 파이프 문자 | 게이트 spec에 리터럴 파이프 문자를 쓰지 않는다. |
| `.issueops/TESTING.md` 최소 완료 기준 | 문서만 바꿔도 기본 게이트 | build, docs, inspect, self-verify를 게이트에 둔다. |
| `.issueops/COMMIT_POLICY.md` | Conventional + Lore | 커밋 메시지는 이 정책을 따른다. |

## 재사용하는 기존 구현

- 규칙의 근거는 이미 코드와 문서에 있다. 새 규칙을 만들지 않고, 스킬 문구와 ADR 기록을 기존 규칙에 맞춘다.
- ADR 추가·색인 갱신은 저장소가 정한 도구(`project_docs_append`, `project_docs_revise`)를 쓴다.
- 스킬 검증은 기존 `scripts/validate-skill.py`, `scripts/verify-skill-shell.py`를 쓴다.

## 성능 영향

- 없음. 문서와 스킬 문구만 바뀐다. `claude -p` 3라운드는 리뷰가 3라운드까지 갈 때만 실행된다.

## 하위 호환성과 side effect

- CLI JSON, MCP schema, record 스키마, response-contract golden은 바뀌지 않는다.
- 설치된 스킬(`~/.claude/skills/issueops-review/SKILL.md`)은 머지 뒤 `issueops update`로 갱신된다. 그 전까지는 저장소의 새 문구가 적용되지 않는다.
- codex와 omo의 3라운드 동작은 그대로다. 새 하위 항목은 Claude에만 적용된다.
- 롤백: 커밋을 되돌리면 된다.
- LLM 프롬프트 본문 변경: 리뷰 스킬의 절차 산문을 바꾸지만 서브에이전트에게 주는 프롬프트 템플릿은 바꾸지 않는다. 측정 기준은 "3라운드 리뷰어가 Fable로 실행되지 않는다"이며, 스킬 문구(G3)로 확인한다.

## Work Objectives

### Core Objective

리뷰 3라운드에서 에이전트가 Fable 5를 스스로 고르지 않게 하고, Claude에서 effort를 올리는 방법을 스킬에 적는다. 옛 ADR의 Claude 기본값 표기는 새 기록으로 대체한다.

### Deliverables

- `skills/issueops-review/SKILL.md` 3라운드 규칙 문단과 "나쁜 예" 한 항목
- 새 ADR 1개(`.issueops/adr/2026-09-24-<slug>.md`)
- `.issueops/adr/README.md` 대체 목록 한 항목, `.issueops/ADR.md` 색인 한 행
- 게이트 원장 `.issueops/issues/517/gates.md`

### Definition of Done

- 게이트 G1~G13이 한 run에서 모두 통과한다.

### Must Have

- 3라운드 규칙이 Fable 5를 3라운드용으로 고르지 않는다고 명시한다.
- Claude의 3라운드 실행 방법(`claude -p --model ... --effort ...`)이 적혀 있다.
- 새 ADR이 2026-07-24 기록의 Claude 기본값 표기를 대체한다고 밝힌다.

### Must NOT Have

- 코드의 모델 기본값 변경(`internal/port/orca.go`, `internal/contract/issueopspreparation/prepare.go`)
- 2026-07-24 ADR 파일 수정
- codex·omo의 3라운드 규칙 변경
- 과거 이슈 기록, verified-execution 기록, record의 리뷰 판정 수정

## Verification Strategy

> 모든 검증은 에이전트가 실행한다.

- 게이트 원장: `.issueops/issues/517/gates.md`. 구현 진입 때 아래 spec 하나로 `gates init`을 실행하고, `issueops gates check --write --timeout-seconds 900`으로 한 run에 채운다.
- 시간 제한 근거(R1-3): 명령 정책의 상한은 15분이다(`internal/adapter/policy/policy_evaluate.go:74`). 2026-09-24 origin/main 사본에서 CI의 검증 단계를 재현했을 때, 다른 세션들이 Go 테스트를 동시에 돌리는 부하 속에서도 `go test ./... -count=1`은 242초, `go test -race ./... -count=1`은 547초였다. 그래서 G10은 900초 안에 3배 넘는 여유가 있다. G10이 시간 초과로 실패하면 부하를 확인한 뒤 같은 run을 다시 실행하고, 그래도 넘으면 `./cmd/...`와 `./internal/...` 두 게이트로 나눈다.
- G13 근거와 처리 규칙(R2-1): self-verify의 risk QA는 바뀐 경로에 `.go` 파일이 없으면 race·static 단계를 건너뛴다(`cmd/issueops/riskqa/risk_qa_plan.go:19-49`, 사유 문구 "no Go changes detected; race/static tier skipped"). 이 사이클은 `skills/`와 `.issueops/`의 `.md`만 바꾸므로 G13은 race를 돌리지 않는다. 위 로컬 재현의 `FAIL self-verify (819s)`는 Go 테스트 파일 변경으로 elevated tier가 되어, self-verify 내부 race 테스트가 10분 제한(`cmd/issueops/riskqa/risk_qa.go:15`)을 부하 속에서 넘은 결과다(`risk QA race test: timeout after 10m0s`). 같은 변경을 올린 2026-09-24 GitHub CI(run 35962833554, 커밋 8e82edea)에서는 seed 고정 self-verify 게이트가 통과했다. 처리 규칙: 원장을 봉인하기 전에 G13 명령을 워크트리에서 1회 실행해 소요 시간과 판정을 구현 보고에 적는다. 600초를 넘거나 실패하면 출력으로 원인을 가린다. 환경 부하나 외부 도구 문제면 부하가 줄어든 뒤 원장 전체를 한 run으로 다시 실행한다. 이 변경이 원인이면 고친 뒤 다시 실행한다. 어느 경우에도 서로 다른 run의 부분 통과를 조합하지 않는다(`.issueops/TESTING.md` 부분 검증 상태 금지).
- 구현 리뷰(`issueops-review --target diff`)는 fresh subagent가 판정한다. 3라운드까지 가면 이 계획이 추가하는 규칙대로 `issueops next`의 `review.model`을 `review.effort`보다 한 단계 높은 effort로 `claude -p` 새 세션에서 띄운다. 이 사이클의 변경은 docs-only로 분류되므로 `review.effort`는 `medium`일 수 있다.

### 게이트 (gates-ledger `--gate` 형식)

```text
G1: 리뷰 스킬 검증 | CHECK: python3 scripts/validate-skill.py skills/issueops-review
G2: 리뷰 스킬 셸 블록 검증 | CHECK: python3 scripts/verify-skill-shell.py skills/issueops-review
G3: 3라운드 규칙에 Fable 5 제외와 claude -p 실행 방법이 있다 | CHECK: python3 -c "import sys; s=open('skills/issueops-review/SKILL.md',encoding='utf-8').read(); i=s.find('같은 대상의 수정·재리뷰는 최대 3라운드다'); block=s[i:i+1200] if i>=0 else ''; need=['Fable 5','claude -p --model','--effort','next.review.effort','codex']; miss=[n for n in need if n not in block]; print('present' if not miss else 'missing: '+','.join(miss)); sys.exit(1 if miss else 0)" | EXPECT: present
G4: 나쁜 예에 Fable 3라운드 항목이 있다 | CHECK: python3 -c "import sys; s=open('skills/issueops-review/SKILL.md',encoding='utf-8').read(); i=s.find('## 나쁜 예'); ok=i>=0 and 'Fable 5' in s[i:]; print('present' if ok else 'missing'); sys.exit(0 if ok else 1)" | EXPECT: present
G5: 새 ADR이 옛 Claude 기본값을 대체한다고 밝힌다 | CHECK: python3 -c "import glob,sys; fs=[f for f in glob.glob('.issueops/adr/2026-09-2*.md') if 'Fable 5' in open(f,encoding='utf-8').read() and '2026-07-24' in open(f,encoding='utf-8').read()]; print('adr_present' if fs else 'adr_missing'); sys.exit(0 if fs else 1)" | EXPECT: adr_present
G6: 2026-07-24 ADR 파일은 그대로다 | CHECK: git diff --exit-code 92bbbddabb9bbee9c7a1e050fc6fe061d7d45201 -- .issueops/adr/decisions/2026-07-24-issueops-planner-implementer-dual-structure.md
G7: README 대체 목록과 ADR 색인이 새 ADR 파일을 가리킨다 | CHECK: python3 -c "import glob,os,sys; fs=[f for f in glob.glob('.issueops/adr/2026-09-2*.md') if 'Fable 5' in open(f,encoding='utf-8').read() and '2026-07-24' in open(f,encoding='utf-8').read()]; idx=open('.issueops/ADR.md',encoding='utf-8').read(); r=open('.issueops/adr/README.md',encoding='utf-8').read(); ok=bool(fs) and any('adr/'+os.path.basename(f) in idx for f in fs) and '2026-07-24' in r and 'Fable 5' in r; print('linked' if ok else 'not_linked'); sys.exit(0 if ok else 1)" | EXPECT: linked
G8: 코드의 모델 기본값은 그대로다 | CHECK: git diff --exit-code 92bbbddabb9bbee9c7a1e050fc6fe061d7d45201 -- internal/port/orca.go internal/contract/issueopspreparation/prepare.go
G9: 응답 계약 golden | CHECK: go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
G10: Go 전체 테스트 | CHECK: go test ./... -count=1
G11: 바이너리 빌드 | CHECK: go build -o bin/issueops ./cmd/issueops
G12: docs·inspect 기본 게이트 | CHECK: python3 -c "import subprocess,sys; rs=[subprocess.run(['./bin/issueops',c,'--json'],capture_output=True).returncode for c in ('docs','inspect')]; print('ok' if rs==[0,0] else 'rc='+str(rs)); sys.exit(0 if rs==[0,0] else 1)" | EXPECT: ok
G13: self-verify 기본 게이트 | CHECK: ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

## Execution Strategy

- Wave 1: T1(스킬), T2(ADR과 색인). 둘은 서로 독립이지만 한 세션이 순서대로 한다.

| Task | Depends On | Blocks | Can Parallelize With |
|---|---|---|---|
| T1 | — | — | T2 |
| T2 | — | — | T1 |

## TODOs

- [ ] T1. 리뷰 스킬의 3라운드 규칙 고치기

  **What to do**:
  1. `skills/issueops-review/SKILL.md`의 "루프 규칙" 절에서 "같은 대상의 수정·재리뷰는 최대 3라운드다."로 시작하는 항목(144~147행) 바로 아래에 하위 항목 두 개를 넣는다. 원래 항목의 문장은 그대로 둔다.

     ```markdown
       - Claude에서 "다른 모델"은 사용자가 이름으로 지정한 모델만 쓴다. Fable 5는 명시적
         수동 지정 전용이므로(`internal/contract/issueopspreparation/prepare.go`) 3라운드용으로
         고르지 않는다. codex와 omo는 이 항목의 적용을 받지 않는다.
       - Claude는 3라운드에도 `next.review.model`을 쓰고, effort는 `next.review.effort`에서
         한 단계 올린다. claude CLI의 단계는 `low`→`medium`→`high`→`xhigh`→`max`다.
         서브에이전트 도구는 effort를 받지 않으므로, 프롬프트를 표준 입력으로 넘겨
         `claude -p --model "$REVIEW_MODEL" --effort "$ROUND3_EFFORT" --allowedTools
         "Bash Read Grep Glob Skill"`로 빈 컨텍스트 세션을 띄우고 출력은 파일로 받는다.
         `--reviewer-model`·`--reviewer-effort`와 finding 첫 줄에는 실제로 넘긴 두 값을 적는다.
     ```
  2. 같은 파일의 "## 나쁜 예" 목록 끝에 한 항목을 더한다: "- 3라운드에서 '다른 모델'로 Fable 5를 고른다. Fable 5는 사용자가 이름으로 지정할 때만 쓴다."
  3. `python3 scripts/validate-skill.py skills/issueops-review`, `python3 scripts/verify-skill-shell.py skills/issueops-review`를 실행한다.

  **Must NOT do**: 3라운드 항목의 원래 문장, 다른 절, 다른 스킬을 바꾸지 않는다. #513이 고치는 128행 근처와 177행 근처 문단을 건드리지 않는다.

  **Recommended Agent**: quick
    Reason: 한 파일에 문단 두 개와 한 줄을 더하는 문서 수정이다.

  **References**: `skills/issueops-review/SKILL.md:130-160`(루프 규칙), `:183-`(나쁜 예), `claude --help`의 `-p`, `--model`, `--effort`, `--allowedTools`

  **Acceptance Criteria**:
  - [ ] G1~G4 통과

  **QA Scenarios**:

  ```
  Scenario: 규칙 문구
    Channel: bash
    Steps: G3과 G4의 CHECK를 실행한다
    Expected: 각각 present, 종료 코드 0
    Evidence: .issueops/issues/517/gates.md의 G3·G4 EVIDENCE

  Scenario: 수정 전 확인
    Channel: bash
    Steps: 수정 전에 G3의 CHECK를 실행한다
    Expected: missing: Fable 5,claude -p --model,--effort,next.review.effort,codex 와 종료 코드 1
    Evidence: 구현 보고
  ```

  **Commit**: YES | Message: `docs(review): keep Fable 5 out of round-3 reviewer choice` | Files: `skills/issueops-review/SKILL.md`

- [ ] T2. Claude 역할 모델 기본값 ADR 추가

  **What to do**:
  1. `project_docs_append(kind=adr)`(MCP) 또는 `issueops project append --kind adr`로 새 ADR을 만든다. 파일은 `.issueops/adr/2026-09-24-<slug>.md`에 생긴다. 내용은 다음과 같다.
     - Title: `Claude role models: Opus 5 plans and reviews, Sonnet 5 implements, Fable 5 is manual-only`
     - Source: `user feedback 2026-09-24 (#517)`
     - Summary: "Claude host의 역할 모델 기본값을 기록한다. 계획과 리뷰는 claude-opus-5/high, 구현은 claude-sonnet-5/high가 맡고, Fable 5는 사용자가 이름으로 지정할 때만 쓴다. 2026-07-24 planner/implementer 기록이 적은 Claude 기본값(fable5/opus4.8)을 대체한다."
     - Context: "2026-07-24 ADR이 적은 Claude 역할 모델 기본값은 이후 코드에서 바뀌었지만 대체 기록이 없었다. 리뷰 스킬의 3라운드 규칙은 '다른 모델 또는 한 단계 높은 effort'만 요구해서, 2026-09-24 #513과 #514의 계획 리뷰 3라운드에서 에이전트가 Fable 5를 골랐다. 사용자는 Fable을 명시적으로 지정할 때만 쓰라고 지적했다."
     - Decision: "Claude 기본값은 코드가 소유한다(internal/port/orca.go의 IssueOpsPlannerModelClaude, internal/contract/issueopspreparation/prepare.go의 ImplementerModelClaude). Fable 5는 자동 기본값, 폴백, 리뷰 3라운드의 대체 모델로 쓰지 않는다. Claude의 리뷰 3라운드는 next.review.model을 쓰고 effort를 next.review.effort에서 한 단계 올려 claude -p 새 세션으로 실행한다."
     - Consequences: "issueops-review 스킬의 3라운드 규칙이 이 결정을 따른다. 2026-07-24 기록의 planner/implementer 이원 구조는 그대로다. 이미 기록된 리뷰 판정(#513 계획 리뷰 3라운드)은 바꾸지 않는다."
     - 도구가 Evidence와 Alternatives를 받으면 다음을 넣는다. Evidence: `internal/contract/issueopspreparation/prepare.go:18-19`, `internal/port/orca.go:19`, `skills/issueops-review/SKILL.md`. Alternatives: "Fable 5를 3라운드 대체 모델로 허용한다: 명시적 수동 지정 규칙과 충돌해 기각", "3라운드에서도 같은 effort의 같은 모델을 쓴다: 규칙이 요구하는 관점 변화가 없어 기각".
  2. `project_docs_revise`로 `.issueops/adr/README.md`의 "Status model" 대체 목록 끝에 다음 항목을 더한다: "- The Claude role-model defaults named in the 2026-07-24 planner/implementer record (claude fable5/opus4.8) are superseded by the 2026-09-24 Claude role-model decision: planner and reviewer `claude-opus-5`/high, implementer `claude-sonnet-5`/high, and Fable 5 only on explicit manual request. The dual planner/implementer structure itself is unchanged."
  3. `project_docs_revise`로 `.issueops/ADR.md`의 Decision index 맨 위에 한 행을 더한다: `| 2026-09-24 | Claude role models: Opus 5 plans and reviews, Sonnet 5 implements, Fable 5 is manual-only; supersedes the 2026-07-24 Claude defaults | [record](adr/<새 파일 이름>) |`. append 도구가 색인을 이미 갱신했다면 그 행의 문구만 이 문장과 맞춘다.
  4. `go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -update -count=1`을 실행하고 `git diff cmd/issueops/testdata/response_contracts.golden.json`이 비어 있는지 확인한다. 현재 투영에서는 비어 있어야 한다(Gap Analysis 4). 비어 있지 않으면 커밋하지 않고 원인을 찾는다.

  **Must NOT do**: 2026-07-24 ADR 파일을 고치지 않는다. 다른 ADR과 문서를 고치지 않는다. golden 파일은 바꾸지 않는다.

  **Recommended Agent**: quick
    Reason: 정해진 도구로 기록 하나를 추가하고 색인 두 곳을 고치는 일이다.

  **References**: `.issueops/adr/README.md`(Status model, Naming and authoring), `.issueops/ADR.md`(Decision index), 형식 예시 `.issueops/adr/2026-09-23-issueops-mcp-serves-in-process-the-shared-daemon-leaves-the.md`

  **Acceptance Criteria**:
  - [ ] G5~G9 통과

  **QA Scenarios**:

  ```
  Scenario: ADR과 색인
    Channel: bash
    Steps: G5, G6, G7의 CHECK를 실행한다
    Expected: adr_present, 종료 코드 0, linked
    Evidence: .issueops/issues/517/gates.md의 G5~G7 EVIDENCE

  Scenario: golden 불변
    Channel: bash
    Steps: G9의 CHECK를 실행하고 git diff --exit-code -- cmd/issueops/testdata/response_contracts.golden.json을 실행한다
    Expected: 둘 다 종료 코드 0
    Evidence: G9 EVIDENCE와 구현 보고
  ```

  **Commit**: YES | Message: `docs(adr): record Claude role-model defaults and manual-only Fable 5` | Files: 새 ADR, `.issueops/adr/README.md`, `.issueops/ADR.md`

## Final Verification Wave

- [ ] F1. Plan Compliance Audit: T1과 T2를 계획대로 했고 Must NOT Have를 어기지 않았다.
- [ ] F2. Code Quality Review: 문구가 코드의 기존 규칙과 모순되지 않는다.
- [ ] F3. Real Manual QA: G1~G13이 한 run에서 통과했고 EVIDENCE가 원장에 있다.
- [ ] F4. Scope Fidelity Check: 코드 기본값(G8)과 2026-07-24 ADR(G6)이 그대로다.

## Commit Strategy

- `.issueops/COMMIT_POLICY.md`의 Conventional + Lore 형식을 따른다.
- C1 `docs(review): keep Fable 5 out of round-3 reviewer choice`: T1
- C2 `docs(adr): record Claude role-model defaults and manual-only Fable 5`: T2
- C3 `chore(issueops): record the #517 gate ledger`: `.issueops/issues/517/gates.md`
- 커밋과 푸시는 이 사이클의 이슈 브랜치 `517-review-round3-no-fable`에만 한다.

## Success Criteria

- intent의 성공 기준 4개가 G1~G13으로 확인된다.
- draft PR이 `main`을 대상으로 발행되고, `issueops execution complete`가 기록된다.
