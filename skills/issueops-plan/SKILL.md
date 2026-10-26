---
name: issueops-plan
description: 'Prepare the plan, reviews, and canonical worktree of an IssueOps cycle and hand off implementation. Use when `issueops next` reports plan.*, or for "아키텍처 정해줘", "계획 세워줘", "계획 검토해줘", "구현 인계".'
---

# IssueOps Plan

이 스킬의 일은 **아키텍처를 먼저 정하고, 그 위에 구현할 수 있는 계약을 만들어 구현 세션에 넘기는 것**이다. 구현은
하지 않는다. 워크트리를 먼저 준비하고, 환경별 자동 세션 인계로 구현에 넘긴다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 계획 작성: [`implementation-planning`](../implementation-planning/SKILL.md)
- 적대 리뷰(아키텍처 문서와 계획): [`issueops-review`](../issueops-review/SKILL.md)
- 게이트 원장: [`gates-ledger`](../gates-ledger/SKILL.md)
- 프로젝트 문서 갱신 절차: [`project-docs-update`](../project-docs-update/SKILL.md)
- 다음 단계: [`issueops-implement`](../issueops-implement/SKILL.md)

## 이 스킬이 맞는지 확인

```bash
issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `plan.architecture`, `plan.write`, `plan.review`, `plan.handoff` 중 하나면 이
스킬이다. 네 값은 이 스킬 안의 어느 절부터 시작할지를 말한다. 순서는 이 표기 그대로이고,
아키텍처가 승인되기 전에는 계획 단계로 넘어가지 못한다. 계획이 먼저 스테이징돼 있어도
`next`는 `plan.architecture`를 돌려준다. `prepare`면
[`issueops-prepare`](../issueops-prepare/SKILL.md)가 먼저다.

## 어디에서 실행하는가

이 단계는 **source checkout의 준비 세션**이 수행한다. 워크트리는 아직 없다. 워크트리를
만드는 것은 이 단계 끝의 `execution prepare`다. 준비한 뒤 환경별 자동 세션 인계를 적용한다.

그래서 아키텍처 문서와 계획 파일은 **source checkout 밖의 임시 파일**에 쓰고 `artifact stage`로
올린다. source checkout 안에 만들면 그 파일이 커밋 대상이 되고, 워크트리가 생긴 뒤에는
두 곳에 같은 문서가 존재한다.

`git worktree add`를 실행하지 않는다.

## 프로젝트 문서 확인

아키텍처를 쓰기 **전에** 이 변경을 제약하는 운영 문서를 읽는다. 아키텍처와 계획을 다 쓰고 나서
읽으면 이미 내려진 결정을 되돌리게 된다.

```bash
# MCP: project_docs_route에 이슈 제목과 구현 범위를 넣어 읽을 문서를 고른다.
# MCP를 쓸 수 없으면 같은 라우터를 CLI로 실행한다.
issueops project route-docs --repo "$SOURCE_ROOT" --task "<이슈 제목과 구현 범위>" --json
# MCP: project_docs_read로 각 문서의 현재 내용과 SHA를 읽는다.
```

route 결과는 아래 목록에 **더하는** 후보이고 목록을 대신하지 않는다. route가 무엇을 돌려주든
다음 여섯 문서는 직접 열고, 색인이 이 변경의 모듈을 가리키면 그 모듈 파일도 연다:
`CONSTITUTION.md`, `ARCHITECTURE.md`, `CONVENTIONS.md`, `CAUTIONS.md`, `ADR.md`, `TESTING.md`.

여섯 문서와 route가 더한 문서마다 한 줄씩 적용 여부를 계획의 `## 적용되는 결정과 주의사항` 절에 적는다. 여섯 문서는 "열지 않음"을 쓸 수 없고, 한 줄이라도 빠지면 읽지 않은 것이다. 저장소에 없는 문서는 "파일 없음"으로 적는다. 한 줄의 형식, "대조했으나 없음"과 "열지 않음"을 쓰는 조건, 이 절이 흘러가는 곳은 [세부 규칙](references/project-docs-check.md)을 연다.

## 아키텍처 작성과 스테이징

계획보다 먼저 **아키텍처 문서**를 쓴다. 문서는 구현 순서가 아니라 구조와 경계에 대한 결정을
담는다. 저장 위치는 `$(mktemp -d)/architecture.md`처럼 source checkout 밖이다.

다섯 절(`## 요구사항과 규모`, `## 데이터와 API 계약`, `## 구조와 흐름`, `## 병목과 장애`, `## 절충점과 기각한 대안`)이 반드시 있고 본문이 비어 있지 않아야 한다(관계없는 절은 "해당 없음: <근거>"). 절별 내용과 거부 규칙은 [필수 절](references/architecture-and-plan-sections.md)을 연다.

```bash
issueops artifact stage --id "$ISSUEOPS_ID" --name architecture --file "$TMP_ARCHITECTURE" --json
```

스테이징한 문서는 `artifact unstage --name architecture`로 내리거나 같은 이름으로 다시 올려
교체한다. 승인한 뒤 문서를 고치면 승인이 `architecture_review_stale`이 되므로 다시 검토한다.

## 아키텍처 검토

[`issueops-review`](../issueops-review/SKILL.md)를 `--target architecture`로 호출한다. 리뷰어는 빈
컨텍스트의 서브에이전트이고 [`architecture-review`](../architecture-review/SKILL.md)의 아키텍처
렌즈를 적용한다. 루프 절차는 그 스킬이 소유한다. `pass` 판정일 때만 승인을 기록한다.

```bash
issueops architecture review --id "$ISSUEOPS_ID" \
  --problem-summary "<문제>" --proposed-architecture "<아키텍처 결정>" --refactor-plan "<리팩터 계획>" \
  --alternative "<기각한 대안과 이유>" --risk "<위 문서 확인에서 온 제약>" \
  --verification "아키텍처 검토로 대안과 위험을 확인했다" \
  --reviewer-context subagent --reviewer-host "$HOST" --reviewer-model "$REVIEWER_MODEL" --reviewer-effort "$REVIEWER_EFFORT" \
  --approved $RECORD_ACTOR_FLAGS --json

issueops record-routing --id "$ISSUEOPS_ID" --phase plan --skill implementation-planning \
  $RECORD_ACTOR_FLAGS --json
```

- 승인은 스테이징한 아키텍처 문서의 sha256에 묶인다. 문서가 없거나 필수 절이 비면 기록이
  거부된다.
- `--verification`에는 "아키텍처"와 "검토"가 함께 들어가야 한다(영어면 "architecture"와
  "review"). 그 두 단어가 없으면 `architecture_review_evidence`가 통과하지 않는다.
- `--reviewer-context`는 `subagent` 또는 `inline`인 감사 필드다. 독립 서브에이전트가 돌려준 판정만 `subagent`로 기록한다. 저자 세션의 검토는 사용자가 inline 리뷰를 명시적으로 지시했을 때만 `inline`으로 기록하고, 그 외에는 리뷰가 아니라 규칙 위반이다. 이유는 [세부 규칙](references/gates-and-review-details.md)을 연다.
- `--refactor-plan`, `--alternative`, `--risk`는 각각 하나 이상 필요하고, open question은
  0이어야 승인된다. 열린 질문이 남았으면 그것을 먼저 닫는다.
- `revise`면 문서를 고쳐 다시 스테이징하고 다시 검토한다. `stop`이면 승인을 기록하지 않고
  이슈 본문의 사실 변경을 `feedback add --classification contract_change`로 남긴 뒤 이슈를
  동기화한다.

## 계획 작성과 스테이징

승인된 아키텍처가 있어야 이 절로 넘어온다. [`implementation-planning`](../implementation-planning/SKILL.md)을
호출해 계획을 쓴다. 계획은 아키텍처에서 내린 결정을 실행 순서로 풀어 쓴다. 쓰다가 아키텍처를
바꿔야 하면 계획을 멈추고 `plan.architecture`로 돌아가 문서를 고치고 다시 검토한다.
저장 위치는 `$(mktemp -d)/plan.md`처럼 source checkout 밖이다.

계획에는 네 절(`## 적용되는 결정과 주의사항`, `## 재사용하는 기존 구현`, `## 성능 영향`, `## 하위 호환성과 side effect`)이 반드시 있다. 절별 내용과 LLM 프롬프트 변경 기준은 [필수 절](references/architecture-and-plan-sections.md)의 "계획"을 연다. 제목을 바꾸거나 합치지 않는다. 네 제목 중 하나라도 없으면 4단계의 `issueops link-plan`이 `plan is missing required sections`로 연결을 거부한다.

계획에 lifecycle ID, 사용자 요청 범위, 환경별 자동 세션 인계, 승인된 종료점을 적고, 자동 인계는 사용자 요청 범위를 넓히지 않는다. 같은 명령을 게이트와 별도 검증 목록에 중복 등록하지 않는다. 나머지는 [세부 규칙](references/plan-staging-details.md)을 연다.

### 사실 주장 확인

계획이 **기존 동작에 대해 단언하는 문장**은 스테이징 전에 명령으로 확인하고 `파일:라인`을 인용한다. 확인하지 않은 단언은 계획에 쓰지 않고, 확인할 수 없는 것은 미확인 가정으로 적는다. 확인은 저자의 일이다. 무엇을 확인하는지는 [사실 주장 확인](references/fact-claims.md)을 연다.

```bash
issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file "$TMP_PLAN" --json
```

`artifact stage`는 actor 플래그를 받지 않는다. 인수와 내리는 법은 [세부 규칙](references/plan-staging-details.md)을 연다.

`link-plan`은 여기서 하지 않는다. 워크트리가 없어 `plan_in_worktree`를 만족할 수 없다.
prepare가 스테이징한 계획을 워크트리 안에 materialize하고, 4단계가 그것을 링크한다.

## 게이트 원장

계획의 수용 기준을 [`gates-ledger`](../gates-ledger/SKILL.md)의 형식대로 `G1..Gn`
(CHECK/EXPECT)으로 계획 안에 적어 둔다. 게이트마다 `gates init`에 그대로 넘길 한 줄 spec
(`- G1: <결과> | CHECK: <명령> | EXPECT: <기대>`)으로 쓴다. 명령과 기대는 백틱으로 감싸지 않는다(문자 그대로
실행·비교된다). 문자 그대로의 EXPECT는 출력의 한 줄 전체나 줄 머리(뒤에 공백)와만 맞으므로 줄 중간은 `/정규식/`으로
쓰고, 형식은 명령을 한 번 실행해 실제 출력으로 확인한다. CHECK가 읽는 경로는 모두 워크트리 안이어야 한다. 원장 **파일**은 워크트리 안에 있어야 하므로
여기서 만들지 않는다. 4단계가 파일을 만들고 `gates check`로 EVIDENCE를 채운다. 5단계는 정리 뒤
충족된 게이트의 CHECK를 직접 다시 실행하고, 7단계는 `gates status`로 원장을 읽어 증거 유효성을 확인한다.

게이트 중 적어도 하나는 이슈의 완료 조건, 곧 보고된 실패가 사라진 최종 상태를 직접 관측해야 한다. fake를 쓴 unit 테스트는 그 게이트를 대신하지 못한다. smoke 명령과 테스트 관측 규칙은 [세부 규칙](references/gates-and-review-details.md)을 연다.

스테이징 전에 계획의 게이트를 실행 없이 검사한다. `problems`가 남은 계획은 스테이징하지 않는다.

```bash
issueops gates lint --plan "$TMP_PLAN" --cwd "$SOURCE_ROOT" --json
```

## 검토 루프

[`issueops-review`](../issueops-review/SKILL.md)를 `--target plan`으로 호출한다. 루프
절차는 그 스킬이 소유하므로 여기 복사하지 않는다. 이 단계가 알아야 하는 것은 셋이다.

- **입력**: 스테이징한 계획 전체. 위 `## 적용되는 결정과 주의사항` 절과 승인된 아키텍처 문서 전문이 함께 간다.
- **종료 조건**: `pass` 판정이 finding 하나 이상과 함께 기록됐다. pass도 아래 `reflect-devils-advocate`로 이슈에 반영한다.
- **stop 판정**: 이 단계가 되돌린다.

```bash
issueops remote reflect-devils-advocate --id "$ISSUEOPS_ID" $RECORD_ACTOR_FLAGS --json  # 미리보기
issueops remote reflect-devils-advocate --id "$ISSUEOPS_ID" --confirm $RECORD_ACTOR_FLAGS --json  # 이슈에 반영
issueops regress --id "$ISSUEOPS_ID" --reason "<리뷰 결론>" $RECORD_ACTOR_FLAGS --json
```

`regress`는 사이클을 grill로 되돌리고 아키텍처 승인도 지운다. grill에서 다시 조사한다. 요구가 바뀌면
`feedback add --classification contract_change` → 이슈 본문 갱신(`issueops-sync-issue`) →
`feedback mark-issue-updated` → 새 기준으로 `intent record` 순서로 반영한다. 그 뒤
`phase --to plan`으로 올라와 아키텍처와 계획을 다시 쓰고 다시 검토한다. 판정은 계획 파일의 sha256에 묶이므로,
판정 뒤 계획을 고치면 `devils_advocate_review_stale`이 되어 `next`가 plan.review로 돌아간다. 고쳤으면
다시 검토한다.

plan.handoff에서 사용자가 계획을 바꾸라고 하면 `execution prepare`로 가지 않는다. 바뀐 내용이
아키텍처에 닿으면 아키텍처 문서를 다시 스테이징하고 리뷰를 다시 기록한다. 그다음 계획을 다시
스테이징하고 위 검토를 다시 기록한다. 이슈 본문의 사실이 바뀌면 아래 `contract_change`도 기록한다.

검토가 이슈 본문의 사실(완료 기준, 범위, 원인 설명, 작업 구조)을 틀렸다고 판정하면 계획만 고치고 끝내지 않고 `feedback add --classification contract_change`로 기록한다. 명령 형식과 이후 게이트는 [세부 규칙](references/gates-and-review-details.md)을 연다.

## 인계

사용자 요청이 계획 작성·검토까지라면 여기서 계획과 검토 결과를 보고하고 종료한다.
아래 `execution prepare`, lease 발급, 새 세션 실행이나 구현 진입은 하지 않는다.
이 경우 아래 인계 후 검증 항목도 적용하지 않는다.

```bash
issueops execution whoami --json   # ACTOR_FLAGS 원문
issueops execution prepare --id "$ISSUEOPS_ID" $ACTOR_FLAGS --json        # preview
# preview가 보여 준 workspace root·branch·base를 확인한 뒤, 같은 요청에 --confirm만 더해 실행한다.
```

`--confirm`은 같은 세션에 lease를 부여하고 계획을 materialize한다. 반환된
workspace root, branch, 계획을 확인한 뒤
[`issueops`](../issueops/SKILL.md)의 **환경별 자동 세션 인계**를 적용한다.

비대화형 실행(`claude -p`, `codex exec`, `omp -p`, `opencode run`)은 Herdr가 있어도 인계하지 않고
현재 세션에서 구현으로 이어간다. 판별은 whoami `ancestry[0].pid`의 실행 인자(`ps -o args= -p <pid>`)로 한다.

[session-choice.md](../issueops/references/session-choice.md)에 따라 준비 세션이 Herdr 세션이면
Herdr의 서버·호환성·현재 native host 실행 가능 여부를 확인하고, 사용 가능하면 아래와 같은
release·새 세션 인계 절차를 적용한다.
현재 holder가 결정 기록, 인계 자료 봉인, writer 정리를 마친 뒤 release하고
같은 worktree에 새 세션 하나를 띄운다. 이때 새 쓰기 작업과 하위 작업 dispatch를 중지하고,
소유자, 실행 핸들, 입력 리비전, 쓰기 범위, 결과 위치, 읽기 작업인지 쓰기 작업인지의 분류,
대기 중인 writer가 0이라는 재확인, 자손 프로세스가 실제로 종료됐다는 관측, 늦게 도착한
결과는 격리했다는 기록을 인계 자료와 release 이전 증거로 남긴다.
새 세션은 자기 native actor로 `execution replace --id ID --expected-generation N --preview`를
실행하고, 반환된 `--reseed` 명령을 거쳐 `claim --claim-current-token`으로 lease를 넘겨받는다.
Herdr를 쓸 수 없으면 lease를 유지하고 현재 세션에서 구현으로 이어가거나, 사용자가 직접 연
두 번째 터미널에서 같은 인수 체인으로 수동 인계한다. Herdr는 기존 worktree를 열 뿐
재생성하지 않는다. 진행 방식을 묻지 않는다. 명시적인 보류·범위 제한은 우선한다.

## 의존성과 로컬 설정

워크트리가 생긴 뒤의 규칙이지만 계획에 함께 적는다. 의존성은 canonical worktree 안에서 저장소가 문서화한 설치 명령으로 준비하고, 생성된 의존성 디렉터리나 심볼릭 링크를 커밋하지 않는다. 로컬 전용 설정은 조건을 만족할 때만 링크하고, 추적되는 파일·ignore되지 않은 파일·설명되지 않은 자격 증명 파일은 링크하기 전에 멈춘다. 조건 전문은 [세부 규칙](references/dependencies-and-local-config.md)을 연다.

## 나쁜 예

다음은 금지다: source checkout 안에 계획 파일 만들기, `git worktree add` 실행, worktree 재생성, 확인 없는 단언, 아키텍처 없이 계획 쓰기, 승인·판정 뒤 수정하고 재검토 생략, 리뷰 없이 `--approved`·`--verdict pass` 기록, revise를 `--waive`로 닫기, staged plan 없이 인계, 인계 뒤에도 구현 계속하기. 이유와 대안은 [나쁜 예 표](references/bad-examples.md)를 연다.

## 검증

- `issueops status --id "$ISSUEOPS_ID" --field architecture_review --field devils_advocate_review --json`의 `architecture_review.approved`가
  true이고 `reviewed_architecture_digest`가 현재 아키텍처 문서의 sha256과 같으며, `devils_advocate_review.verdict`가 `pass`이며 그 digest가 현재 계획과 같다.
- `issueops next --id "$ISSUEOPS_ID" --json`의 `stage.key`가 `plan.handoff`
  이거나, 인계 뒤라면 `claim` 또는 `implement.enter`다.
- `git -C "$SOURCE_ROOT" status --short`에 계획 파일이 없다.
- 인계 뒤 `execution prepare` 결과의 `workspace.root`가 실제로 존재하고 그 브랜치가
  record의 브랜치와 같다.
