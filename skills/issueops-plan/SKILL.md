---
name: issueops-plan
description: Prepare the plan, reviews, and canonical worktree for an IssueOps cycle, then automatically hand off to a new session when Orca is ready or continue in the current session otherwise. Use when "issueops next" reports plan.write, plan.design, plan.review, or plan.handoff, or when the user says "계획 세워줘", "계획 검토해줘", "구현 인계".
---

# IssueOps Plan

이 스킬의 일은 **구현할 수 있는 계약을 만들고 구현 세션에 넘기는 것**이다. 구현은
하지 않는다. 워크트리를 먼저 준비하고, 환경별 자동 세션 인계로 구현에 넘긴다.

- 전체 흐름과 단계 판별: [`issueops`](../issueops/SKILL.md)
- 계획 작성: [`implementation-planning`](../implementation-planning/SKILL.md)
- 적대 리뷰: [`issueops-review`](../issueops-review/SKILL.md)
- 게이트 원장: [`gates-ledger`](../gates-ledger/SKILL.md)
- 프로젝트 문서 갱신 절차: [`project-docs-update`](../project-docs-update/SKILL.md)
- 다음 단계: [`issueops-implement`](../issueops-implement/SKILL.md)

## 이 스킬이 맞는지 확인

```bash
issueops next --id "$ISSUEOPS_ID" --json
```

`stage.key`가 `plan.write`, `plan.design`, `plan.review`, `plan.handoff` 중 하나면 이
스킬이다. 네 값은 이 스킬 안의 어느 절부터 시작할지를 말한다. `prepare`면
[`issueops-prepare`](../issueops-prepare/SKILL.md)가 먼저다.

## 어디에서 실행하는가

이 단계는 **source checkout의 준비 세션**이 수행한다. 워크트리는 아직 없다. 워크트리를
만드는 것은 이 단계 끝의 `execution prepare`다. 기본 흐름은 direct로 준비한 뒤 환경별 자동 세션 인계를 적용한다.

그래서 계획 파일은 **source checkout 밖의 임시 파일**에 쓰고 `artifact stage`로 올린다.
source checkout 안에 계획을 만들면 그 파일이 커밋 대상이 되고, 워크트리가 생긴 뒤에는
두 곳에 같은 계획이 존재한다.

`git worktree add`를 실행하지 않는다.

## 프로젝트 문서 확인

계획을 쓰기 **전에** 이 변경을 제약하는 운영 문서를 읽는다. 계획을 다 쓰고 나서
읽으면 이미 내려진 결정을 되돌리게 된다.

```bash
# MCP: project_docs_route에 이슈 제목과 구현 범위를 넣어 읽을 문서를 고른다.
# route를 쓸 수 없으면 required-doc 목록을 쓴다.
issueops docs --json
# MCP: project_docs_read로 각 문서의 현재 내용과 SHA를 읽는다.
```

최소한 다음을 읽는다: `CONSTITUTION.md`, `ARCHITECTURE.md`(해당 모듈),
`CONVENTIONS.md`, `CAUTIONS.md`(색인과 해당 모듈), `ADR.md`(관련 결정), `TESTING.md`.

읽은 결과는 계획의 `## 적용되는 결정과 주의사항` 절에 **문서 경로, 항목 제목, 이 계획에
미치는 제약 한 문장**으로 적는다. 적용되는 항목이 없으면 "대조했으나 없음"과 대조한
문서 목록을 적는다. 읽지 않은 것과 읽었는데 없는 것은 다르다.

이 절은 두 곳으로 흘러간다. design review의 `--risk`에 그 제약이 들어가고,
`issueops-review`의 plan 리뷰 프롬프트에 이 절이 들어가 리뷰어가 "무시된 결정"을
공격한다.

## 계획 작성과 스테이징

[`implementation-planning`](../implementation-planning/SKILL.md)을 호출해 계획을 쓴다. 저장 위치는
`$(mktemp -d)/plan.md`처럼 source checkout 밖이다.

계획에는 다음 네 절이 반드시 있다.

| 절 | 내용 |
|---|---|
| `## 적용되는 결정과 주의사항` | 위 문서 확인의 결과 |
| `## 재사용하는 기존 구현` | plan-prep의 코드베이스 조사에서 찾은 심볼·패키지·테스트 헬퍼와 재사용 방식. 새로 만드는 것이 있으면 기존 것으로 왜 안 되는지 |
| `## 성능 영향` | hot path 여부, 복잡도 변화, 측정 계획. 알고리즘 선택이 걸리면 [`algorithm-optimization`](../algorithm-optimization/SKILL.md) |
| `## 하위 호환성과 side effect` | CLI JSON·MCP schema·golden·record schema·provider body 계약, 기존 데이터, 롤백 경로. 마이그레이션·엔티티·인덱스·쿼리가 걸리면 [`database-design`](../database-design/SKILL.md)으로 정규화 판단·인덱스 계획·예상 row 수를 먼저 정하고 그 값을 여기 적는다. 7단계의 실측이 이 값과 대조된다 |

계획이 **LLM에게 주는 프롬프트 본문**을 바꾸면(서브에이전트·리뷰어 프롬프트,
`.issueops/prompt-engineering/`의 문서, SKILL 안의 프롬프트 템플릿 블록)
[`prompt-engineering`](../prompt-engineering/SKILL.md)으로 측정 가능한 출력 기준을 정해
`## 하위 호환성과 side effect` 절에 적는다: 무엇이 나오면 그 프롬프트가 실패한 것이고,
어떤 산출물로 확인하는가. 메인 에이전트가 읽는 절차 산문 변경은 해당 없다.

이 네 절은 형식이 아니라 판단이다. "재사용할 것이 없다"는 결론도 근거와 함께 적으면
유효하고, 근거 없이 비워 두면 리뷰가 그것을 공격한다. 절 제목 자체는 4단계의
`issueops link-plan`이 검사한다. 네 제목 중 하나라도 없는 계획은
`plan is missing required sections`로 연결이 거부되므로, 제목을 바꾸거나 합치지 않는다.

계획에 lifecycle ID, 사용자 요청 범위, 브랜치·worktree 준비 뒤의 환경별 자동 세션 인계,
승인된 종료점을 적어 Orca owner도 같은 경계를 알게 한다. 자동 인계는 사용자 요청 범위를
넓히지 않는다. 이미 읽은 plan-prep 조사와 문서는 변경이나
새 질문이 없으면 재사용한다. 작업 규모에 맞춰 필요한 검증 명령을 정하고, 같은 명령을
게이트와 별도 검증 목록에 중복 등록하지 않는다.

### 사실 주장 확인

계획이 **기존 동작에 대해 단언하는 문장**은 스테이징 전에 명령으로 확인하고
`파일:라인`을 인용한다. 확인하지 않은 단언은 계획에 쓰지 않는다.

대상은 계획이 "현재 이렇게 되어 있다"고 말하는 모든 것이다. 자주 틀리는 것들:

- 데이터가 어디에 저장되고 무엇을 키로 재사용되는가. "저장 데이터는 그대로"는
  저장 지점과 조회 키를 둘 다 읽기 전에는 쓸 수 없는 문장이다.
- 대상 spec이 내가 고치려는 서비스를 mock하는가. mock하면 그 안에 넣은 변경은
  그 spec으로 관측되지 않으므로 성공 기준을 검증할 수 없다.
- 재시도 횟수·타임아웃·상수의 실제 값과 그 곱. 최악 지연은 곱으로 계산한다.
- 이 동작을 소비하는 호출부가 몇 곳인가. 센 것이 아니라 찾은 것을 적는다.
- 근거로 드는 ADR·선례가 실제로 그 주장을 뒷받침하는가. 제목이 아니라 본문을 읽는다.

이것은 리뷰어의 일이 아니라 저자의 일이다. 확인은 몇 분이지만 틀린 단언 하나는 리뷰
라운드 하나와 계획 전면 개정을 부른다. 적대 리뷰는 저자가 못 보는 설계 결함을 잡으라고
있는 것이지, 저자가 읽지 않은 코드를 대신 읽으라고 있는 것이 아니다.

확인할 수 없는 것은 단언하지 말고 미확인 가정으로 적는다. 그러면 리뷰어가 그 가정을
판별할 최소 확인을 요청할 수 있다.

```bash
issueops artifact stage --id "$ISSUEOPS_ID" --name plan --file "$TMP_PLAN" --json
```

`artifact stage`는 actor 플래그를 받지 않는다. `--id`, `--name`, `--file`, `--json`뿐이다.
잘못 올렸으면 `issueops artifact unstage --id "$ISSUEOPS_ID" --name plan --json`으로
내린다.

`link-plan`은 여기서 하지 않는다. 워크트리가 없어 `plan_in_worktree`를 만족할 수 없다.
prepare가 스테이징한 계획을 워크트리 안에 materialize하고, 4단계가 그것을 링크한다.

## 게이트 원장

계획의 수용 기준을 [`gates-ledger`](../gates-ledger/SKILL.md)의 형식대로 `G1..Gn`
(CHECK/EXPECT)으로 계획 안에 적어 둔다. 원장 **파일**은 워크트리 안에 있어야 하므로
여기서 만들지 않는다. 4단계가 파일을 만들고 `--write`로 EVIDENCE를 채우며, 5단계가
정리 뒤 다시 채우고, 7단계는 증거 유효성을 확인해 읽거나 필요한 검사를 재실행한다.

## 설계 검토

```bash
issueops design review --id "$ISSUEOPS_ID" \
  --problem-summary "<문제>" --proposed-design "<설계>" --refactor-plan "<리팩터 계획>" \
  --alternative "<기각한 대안과 이유>" --risk "<위 문서 확인에서 온 제약>" \
  --verification "설계 검토로 대안과 위험을 확인했다" --approved $RECORD_ACTOR_FLAGS --json

issueops record-routing --id "$ISSUEOPS_ID" --phase plan --skill implementation-planning \
  $RECORD_ACTOR_FLAGS --json
```

- `--verification`에는 "설계"와 "검토"가 함께 들어가야 한다(영어면 "design"과 "review").
  그 두 단어가 없으면 `design_review_evidence`가 통과하지 않는다.
- `--refactor-plan`, `--alternative`, `--risk`는 각각 하나 이상 필요하고, open question은
  0이어야 승인된다. 열린 질문이 남았으면 그것을 먼저 닫는다.

## 검토 루프

[`issueops-review`](../issueops-review/SKILL.md)를 `--target plan`으로 호출한다. 루프
절차는 그 스킬이 소유하므로 여기 복사하지 않는다. 이 단계가 알아야 하는 것은 셋이다.

- **입력**: 스테이징한 계획 전체. 위 `## 적용되는 결정과 주의사항` 절이 포함된다.
- **종료 조건**: `pass` 판정이 finding 하나 이상과 함께 기록됐다.
- **stop 판정**: 이 단계가 되돌린다.

```bash
issueops regress --id "$ISSUEOPS_ID" --reason "<리뷰 결론>" $RECORD_ACTOR_FLAGS --json
```

`regress`는 사이클을 grill로 되돌린다. 다시 조사하고 이슈 본문을 갱신한 뒤
`phase --to plan`으로 올라와 계획을 다시 쓴다. 판정은 계획 파일의 sha256에 묶이므로,
판정 뒤 계획을 고치면 `devils_advocate_review_stale`이 되어 인계가 막힌다. 고쳤으면
다시 검토한다.

## 인계

사용자 요청이 계획 작성·검토까지라면 여기서 계획과 검토 결과를 보고하고 종료한다.
아래 `execution prepare`, lease 발급, 새 세션 실행이나 구현 진입은 하지 않는다.
이 경우 아래 인계 후 검증 항목도 적용하지 않는다.

```bash
issueops execution whoami --json   # ACTOR_FLAGS 원문
issueops execution prepare --id "$ISSUEOPS_ID" --mode direct \
  --direct-reason "워크트리 준비 후 환경별 자동 세션 인계" \
  --owner-host "$HOST" $ACTOR_FLAGS --json        # preview
# 출력의 next_command(--expected-readiness-fingerprint 포함)를 그대로 실행한다.
```

이 기본 경로는 기존 direct API를 사용해 같은 세션에 lease를 부여하고 계획을
materialize한다. 반환된 `resolved_mode`, canonical path, branch, 계획을 확인한 뒤
[`issueops`](../issueops/SKILL.md)의 **환경별 자동 세션 인계**를 적용한다.

[session-choice.md](../issueops/references/session-choice.md)에 따라 Orca가 ready면
현재 holder가 결정 기록과 release를 마치고 같은 worktree에 새 세션 하나를 띄운다.
Orca가 없거나 unready면 lease를 유지하고 현재 세션에서 구현으로 이어간다.
실행 방식이나 진행 여부를 묻지 않는다. 명시적인 보류·범위 제한은 우선한다.

사용자가 명시적으로 Orca execution을 요청했거나 기존 사이클이 Orca mode면 그 core
경로를 보존한다. `auto|orca` API의 의미는 바꾸지 않는다. planner gate 실패를 mode 변경으로
우회하지 않고 필요한 기록을 채운다. 단순히 Orca가 있다는 이유로 그 경로를 선택하지 않는다.

## 의존성과 로컬 설정

워크트리가 생긴 뒤의 규칙이지만 계획에 함께 적는다.

의존성은 canonical worktree 안에서 저장소가 문서화한 설치 명령으로 준비한다. 크게
생성되는 의존성 디렉터리를 재사용하려면 패키지 매니저·lockfile·런타임·플랫폼·네이티브
모듈 상태가 모두 같은지 확인한 뒤에만 한다. 생성된 의존성 디렉터리나 심볼릭 링크를
커밋하지 않는다.

`.env`, `.env.local`, `.mcp.json`, `dbhub.toml` 같은 로컬 전용 설정은 작업에 필요하고,
원본이 ignore돼 있고, 어떤 secret도 프롬프트·로그·테스트·이슈 본문·PR/MR 본문에 들어가지
않을 때만 링크한다. 추적되는 파일, ignore되지 않은 파일, 설명되지 않은 자격 증명 파일은
링크하기 전에 멈춘다.

## 나쁜 예

| 나쁜 행동 | 왜 나쁜가 | 대신 할 일 |
|---|---|---|
| source checkout 안에 계획 파일을 만든다 | 커밋 대상이 되고 워크트리 생성 뒤 계획이 두 곳에 존재한다 | 임시 디렉터리에 쓰고 `artifact stage` |
| `git worktree add`를 실행한다 | Orca 경로가 이름 충돌로 깨진다 | `execution prepare`가 만들게 둔다 |
| 세션 인계를 위해 `--mode auto`로 바꾸거나 worktree를 재생성한다 | execution mode와 native 세션 인계를 혼동한다 | direct로 준비한 같은 worktree에서 환경별 자동 세션 인계를 적용한다 |
| 기존 동작에 대한 단언을 확인 없이 계획에 쓴다 | 저자가 몇 분이면 읽을 코드를 리뷰어가 읽게 되고, 틀린 단언 하나가 리뷰 라운드와 전면 개정을 부른다 | 스테이징 전에 명령으로 확인하고 `파일:라인`을 인용한다 |
| 리뷰 없이 `--verdict pass`를 기록한다 | 게이트 연극이다 | `issueops-review`로 실제 리뷰를 돌린다 |
| revise 판정을 `--waive`로 닫는다 | 지적이 반영되지 않은 채 구현으로 간다 | 계획을 고치고 다시 검토한다 |
| 판정 뒤 계획을 고치고 재검토를 생략한다 | stale 판정으로 인계가 막히거나, 검토되지 않은 계획이 구현된다 | 다시 검토해 새 판정을 기록한다 |
| staged plan 없이 인계한다 | prepare가 워크트리에 materialize할 계획이 없다 | `artifact stage`를 먼저 한다 |
| Orca 세션이 떴는데 이 세션이 계속 구현한다 | 두 세션이 같은 워크트리를 쓴다 | `resolved_mode`가 orca면 여기서 멈춘다 |

## 검증

- `issueops status --id "$ISSUEOPS_ID" --json`의 `design_review.approved`가
  true이고 `devils_advocate_review.verdict`가 `pass`이며 그 digest가 현재 계획과 같다.
- `issueops next --id "$ISSUEOPS_ID" --json`의 `stage.key`가 `plan.handoff`
  이거나, 인계 뒤라면 `claim`(Orca) 또는 `implement.enter`(direct)다.
- `git -C "$SOURCE_ROOT" status --short`에 계획 파일이 없다.
- 인계 뒤 `execution prepare` 결과의 `workspace.root`가 실제로 존재하고 그 브랜치가
  record의 브랜치와 같다.
