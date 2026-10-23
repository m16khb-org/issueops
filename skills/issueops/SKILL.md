---
name: issueops
description: Route an issue-driven work cycle across its ten user-facing stages. Run "issueops next" to decide which stage a cycle is in, hand the work to that stage's skill, and hold the invariants every stage shares. Use when the user asks to start, resume, or continue an issue cycle, when the current stage is unclear, or when they say "이슈옵스", "이슈 작업 시작", "이어서 진행".
---

# IssueOps

IssueOps는 이슈, 브랜치, 계획, 실행 lease, 검증 증거, PR/MR을 durable record
하나로 묶는다. 이 스킬은 **라우터**다. 단계의 일은 단계 스킬이 하고, 이 파일은
어느 단계인지 정하는 방법과 모든 단계가 공유하는 불변식만 소유한다.

이 본문은 복사·단독 주입 때도 적용되는 actor 계약이다. 단계 스킬은 설치되어 있으면
사용하되, 없으면 아래 계약과 CLI가 반환한 `next_command`로 진행한다. 명령·필수 입력을
확인할 수 없으면 그 지점만 보고하고 멈춘다. evaluator 자료나 sibling 설치를 요구하지 않는다.
참고 링크는 cwd가 아니라 symlink를 해소한 이 파일 위치에서 찾는다.

## 먼저 실행

```bash
issueops next --json
```

읽기 전용이며 record·로컬 관측만 쓴다(fetch/provider 호출 없음).

1. 사용자나 현재 대화가 지정한 lifecycle ID가 있으면 처음부터
   `issueops next --id "$ISSUEOPS_ID" --json`을 실행한다. 없을 때만 자동 선택한다.
   `selected.id` 또는 `start` 결과의 ID를 이후 명령·인계·압축 요약에 유지한다.
2. `stage.key`를 아래 `## 단계 표`로 바꾸고 해당 스킬을 실행한다. 단계가 끝나면 같은
   ID로 `next`를 다시 읽고 다음 단계로 이어간다. 단계 선택 메뉴를 반복해서 묻지 않는다.
3. `next_command_kind=template`의 값은 record·whoami·계획·저장소에서 채운다.
   명령의 placeholder나 `--confirm` 자체는 사용자 질문 사유가 아니다.

- `ambiguous`라도 대화에 지정된 ID가 있으면 그 ID로 다시 읽는다. ID가 없고 후보를
  구별할 근거도 없을 때만 고르게 한다. 다른 사이클을 임의로 선택하거나 정리하지 않는다.
- `none`이면 요청 범위에 맞게 이슈 단계로 들어간다. 본문 초안이나 상태 조회만 요청했으면
  그 결과만 제공한다. 사용자가 새 사이클을 요청하면 기존 후보와 별개로 시작한다.
- `blocked.*`는 아래 중단 규칙을 따른다. 다른 holder의 작업을 대신하거나 상태를 우회하지 않는다.

## 환경별 자동 세션 인계

**브랜치·canonical worktree 준비 후, 구현 진입 전**에 실행 세션을 자동 결정한다.
세션 선택 메뉴는 묻지 않는다. 전체 사이클은 draft PR/MR 발행·execution complete에서,
이슈 작성·계획만 요청했거나 더 좁은 종료점을 정했으면 그 범위에서 끝낸다.

- `issueops-plan`이 워크트리를 먼저 준비한다. 준비한 뒤 Herdr를 확인한다. 준비 세션이
  Herdr 세션이고 실행 중인 서버·호환성·현재 native host 실행 가능 여부가 확인되면 같은
  worktree의 새 Herdr 세션으로 인계한다. Herdr를 쓸 수 없으면 현재 세션에서 이어가거나,
  사용자가 직접 연 두 번째 터미널에서 수동 인계한다.
  바이너리 설치 여부만으로 ready라고 판단하지 않는다.
- 결정 기록(`decision add`)·인계 자료·전달 관찰 규칙은 [handoff-observation.md](references/handoff-observation.md)를,
  launcher recipe는 [session-choice.md](references/session-choice.md)를 본문 계약으로 적용한다. 제출 전 staging observation을 기록하고
  입력 수락을 native turn이나 owner claim 증거로 삼지 않으며, 모호한 결과는 reconcile한다. 인계 전 새 dispatch를 멈추고 writer와 자손의 실제 종료를
  확인한 뒤 release/status로 해제를 확인한다. 런처 설치·서버 시작과 모호한 launch의 재전송·우회는 하지 않는다.
  이전 callback은 쓰기나 pass 근거가 아니다.
- 자동 분기는 실행 위치만 정한다. 원래 요청의 승인 범위·종료점을 그대로 유지하며,
  구현만 요청한 작업에 commit·push·PR/MR 발행 권한을 추가하지 않는다.
  phase·claim·`--approved`도 승인 근거가 아니다.
- 현재 세션 진행·새 세션·보류를 명시한 최신 사용자 지시는 자동 분기보다 우선한다.
  보류는 lease를 해제하고 준비 상태를 보존한다. 기존 worktree는
  세션 인계를 이유로 바꾸거나 다시 만들지 않는다.

단계 스킬과 함께 쓰는 계획·검증·Git 스킬도 이 승인 범위와 종료점을 따른다.
하위 스킬의 일반적인 "계속할까요" 절차를 추가 확인 지점으로 만들지 않는다.

## 단계별 협업

단계 라우팅은 아래 표 하나를 쓴다. 필요한 협업만 로드한다:
1단계는 `implementation-planning` 인터뷰·`web-research`, 3단계는 계획·`issueops-review`·
`database-design`·`algorithm-optimization`·`prompt-engineering`, 4단계는 `issueops-debugging`·
`verified-execution`·`ui-ux-craft`·`sync-base`, 5단계는 `code-quality-metrics`·`verified-execution`,
6단계는 `project-docs-update`, 7단계는 `database-design`·`verified-execution`·`aside-web-qa`,
8단계의 history 수술은 `git-operations`다. 공용 협업은 `gates-ledger`(3·4·5·7),
`issueops-review`(3·7), `issueops-remote-write`(1·9·10·동기화), `explain`(사용자 보고)이다.
낡은 본문은 `issueops-sync-issue`/`issueops-sync-pr`로 관리 블록을 보존해 갱신한다.
탈출은 `issueops-abandon`이다. Issue와 PR/MR 생성 스킬을 동시에 읽지 않는다.

## 세션 경계

1·2단계는 worktree 없이 source 준비 세션에서, 3단계도 같은 세션에서 수행한다.
3단계 `execution prepare`가 worktree·lease를 만들고 위 인계를 적용한다.
4단계부터 선택한 세션이 canonical worktree에서 수행한다. current는 기존 lease를 유지하고
새 세션은 release 확인 후 인수한다. 새 세션은 `replace --preview`와 반환된 `--reseed` 명령,
`claim --claim-current-token`으로 이어지는 인수 체인이 유일한 인계 경로다.
host 간 재개·인수도 `next_command`의 generation·quiescence 검증 체인을 따른다.

## 공통 불변식

각 독립 단계의 본문에도 실행에 필요한 최소 계약을 유지한다.

**(a) 단계 판별.** 모든 단계 스킬은 선택한 ID의 `issueops next --id "$ISSUEOPS_ID" --json`으로 시작하고
`stage.key`가 자기 단계인지 확인한다. 아니면 표가 지목하는 스킬로 안내한다. `blocked.*`
면 중단한다. phase를 추정하지 않는다.

**(b) actor 플래그.** `issueops execution whoami --json`이 돌려주는
`record_actor_flags`와 `claim_actor_flags`를 그대로 쓴다. 손으로 조립하지 않는다.

**(c) lease fencing.** durable mutation(phase 전이, record 기록, artifact stage) 전마다
exact lifecycle ID·generation·native actor·canonical cwd를 현재 record와 대조한다.
record 기록은 `RECORD_ACTOR_FLAGS`, lease 전이와 publication은 `ACTOR_FLAGS`를 쓴다.
두 축약의 정의는 `issueops --help`의 legend가 소유한다. 불일치는 stop이다. 사용자의
"그 세션은 내가 껐어"는 quiescence 증거가 아니다. 모호한 결과는 재실행하지 않고 reconcile한다.
lease 전의 준비 기록은 source checkout의 native actor로, 구현은 active holder의 canonical cwd로 한다.

**(d) 편집 대상 확인.** 편집 배치마다 셸 프롬프트를 믿지 말고 실측한다.

`pwd`, branch, HEAD, `test "$PWD" = "$EXPECTED_WORKTREE"`, 두 checkout의 `git status --short`를 실측한다
([명령·복구 절차](references/edit-target-check.md)). 도구가 다른 체크아웃에 썼으면 멈춘다.
worker 프롬프트에는 exact lifecycle ID, generation, branch, worktree, 허용 경로, 수락 기준,
중단 규칙을 넣는다.

**(e) 코드베이스 존중.** 새로 만들기보다 기존 구현의 확장과 재사용을 먼저 보고, 하위 호환성·성능 영향·side effect를
계획·구현 루프·리뷰에서 명시한다([세 곳의 기준](references/codebase-respect.md)).

## Core contract

durable phase는 `problem`, `grill`, `plan`, `compatibility-review`, `implement`,
`ai-slop-clean`, `feedback`, `pr`, `done`이다. `issue`는 linkage, `cleanup`은 done 뒤 후처리다.
한 cycle은 exact ID·canonical worktree·generation-fenced holder·linked Issue·검증된 PR/MR을 갖는다.
CLI `issueops ... --json`/MCP `issueops_execution`이 state를 소유하고 `execution complete`만
done을 기록한다. hook은 SessionStart의 project-doc context만 제공한다. Issue 생성·편집·
테스트·대기·branch/worktree 준비·publication·reply·merge·cleanup을 hook에 맡기지 않는다.

## 단계 표

`stage.key`를 스킬과 label로 바꾸는 유일한 표다. CLI는 이 표를 모른다.

| stage.key | 스킬 | label |
|---|---|---|
| `none`, `issue` | `issueops-create-issue` | 이슈 확정·생성 |
| `prepare` | `issueops-prepare` | 브랜치 준비 |
| `plan.architecture`, `plan.write`, `plan.review`, `plan.handoff` | `issueops-plan` | 문서 확인·아키텍처·계획·검토·인계 |
| `claim` | 스킬 없음. `next_command`가 돌려주는 체인(claimable이면 `claim --claim-current-token`, released이면 `replace --preview`, revoking이면 `replace --finalize-preview`)을 lease가 active(self)가 될 때까지 따라간 뒤 `next`를 다시 실행한다 | 현재 index의 label |
| `implement.enter`, `implement` | `issueops-implement` | 구현 |
| `clean` | `issueops-slop-clean` | AI slop 정리 |
| `docs` | `issueops-docs` | 프로젝트 문서 반영 |
| `verify` | `issueops-verify` | 검증 |
| `commit-push` | `atomic-commit-push` | 커밋·푸시 |
| `pr.create` | `issueops-create-pr` | PR/MR 발행·완료 |
| `pr.complete` | `issueops-complete` | PR/MR 발행·완료 |
| `done` | 머지 전이면 `issueops-merge`, 머지 뒤면 `issueops-cleanup`. 구분은 `issueops remote merge-pr --id ID --json`의 `already_merged`로 한다 | 머지·정리 |
| `takeover` | 스킬 없음. `next_command`를 실행하고 결과가 돌려주는 `next_command`를 따라간다. 죽은 홀더 인수는 `issueops-abandon`이 설명한다 | 현재 index의 label |
| `blocked.pending`, `blocked.holder_live` | 없음. `next_command`로 상태를 다시 읽는다 | 현재 index의 label |
| `blocked.root_conflict` | 충돌 사이클을 `issueops-cleanup`(머지됨) 또는 `issueops-abandon`(미머지)으로 먼저 정리한다 | 현재 index의 label |
| `unknown`, `invalid` | 없음. `next_command`로 record를 읽고 `warnings`의 missing 키를 사용자에게 보여 준다 | 현재 index의 label |
| `ambiguous` | 사용자에게 `candidates` 중 ID 선택 또는 새 사이클 시작을 요청 | 없음 |

`missing`의 해소 명령은 CLI의 `next_command`가 소유한다.

## 구현·검증 규칙

본문만으로 실행해도 evidence schema를 생략하지 않는다. domain 검토·환경 표·feedback classification/resolution·ready/done 보고 전 draft completion record의
필수 항목은 [delivery-evidence-rules.md](references/delivery-evidence-rules.md)에 있다. 완료 보고 전에 읽는다. 테스트 성공만으로 remote 갱신·thread reply·merge readiness·cleanup 완료를 대신하지 않는다.

- behavior change는 focused failing test에서 시작해
  `RED→GREEN→SURFACE→CLEAN` 순서로 검증한다.
- 작업은 canonical worktree에서만 한다. source checkout에 구현하지 않는다.
- API/DTO/OpenAPI 변경은 `.issueops/OPEN_API_SPEC.md` gate를 적용한다.
- live runtime, review reply, completion hygiene가 요청 범위라면 테스트 통과만으로
  완료를 선언하지 않는다.
- ai-slop-clean은 실제 diff가 생긴 뒤 실행하고, cleanup 후 관련 검증을 다시
  실행한다.
- publication 전에 구현 diff를 project docs와 양방향으로 대조하고, 스키마 변경이 있으면 실측 증거를 기록한다.
  세부는 [delivery-evidence-rules.md](references/delivery-evidence-rules.md)에 있다.
- Git staging/push는 `atomic-commit-push`, 고급 history 작업은 `git-operations`가
  소유한다. 원래 요청에서 승인된 issue branch의 commit·push는 다시 묻지 않는다.
- destructive cleanup은 exact target과 fingerprint를 preview한 뒤 별도 사용자
  승인을 받는다.

## 원격 쓰기

`issueops-remote-write`가 설치되어 있으면 사용한다. 단독 실행 순서와 계약 변경 feedback 규칙은
[remote-write-standalone.md](references/remote-write-standalone.md)를 따른다. raw provider 쓰기로
우회하지 않고, 모호한 write는 reconcile하며 중복 생성하지 않는다.

## Reference map

본문으로 라우팅·권한·증거·중단을 판단하고 필요한 recipe만 `references/`에서 읽는다:
`remote-issue.md`(provider 계층), `evidence-contract.md`(domain/API/live/completion),
`execution.md`(lease/claim/recovery/publication), `session-choice.md`(세션 인계),
`orchestration.md`(child), `cleanup-state.md`(머지 후 정리),
`review-feedback.md`(thread; 봇은 `review-agent-feedback`, 사람 리뷰는 `pr-review`).

## Stop conditions

다음 조건이면 해당 phase나 remote write를 실행하지 않는다. 승인 범위 안에서 조사·수정·
재검증으로 해소할 수 있으면 에이전트가 해소하고 같은 ID로 `next`를 다시 읽는다.
stale 판정이나 테스트 실패 자체를 사용자에게 진행 여부를 물을 이유로 삼지 않는다.

- provider, credentials, project, Issue owner가 모호하다. target branch는
  Execution ownership의 base 선택 순서로 해소하고 질문 사유로 삼지 않는다.
  [`issueops-prepare`](../issueops-prepare/SKILL.md)는 추가 설명이며 필수 dependency가 아니다.
- intent·success criteria·domain term 해석이 구현을 바꿀 만큼 갈린다.
- architecture open question, compatibility blocker, stale review(`architecture_review_stale` 포함)가 남아 있다.
- branch/worktree/plan/generation/actor가 current record와 맞지 않는다.
- strict PR readiness가 Issue, branch link, plan, worktree, upstream,
  ai-slop-clean, project-doc 반영 판정, 스키마 실측 근거, contract feedback를
  누락했다고 보고한다.
- label·assignee·한국어 body·target branch·live readback이 검증되지 않았다.
- merge evidence 없이 cleanup을 요청한다.

조사·문서·관례로 답할 수 있으면 진행하고 근거를 보고한다. 질문은 해소되지 않는 요구사항,
승인 범위 변경, 필요한 권한/자격 증명, 자동 복구 불가 충돌에 한정하며 결정과 근거를 적는다.
동일 blocker가 두 번의 복구 뒤에도 남으면 명령·원인을 보고하고 중단한다.
다른 holder를 무한 polling하거나 모호한 원격 mutation을 반복하지 않는다.

## IssueOps benchmark artifact contract

Benchmark 응답에는 의도나 계획만 쓰지 말고 labeled evidence를 넣는다. 5개 라벨과
semantic judge 규칙은 [benchmark-artifact.md](references/benchmark-artifact.md)에 있다.

## Execution ownership

base 선택 순서와 보고 규칙은 [base-selection.md](references/base-selection.md)에 있다. 형제 prepare 스킬이 없어도 그 순서를 적용한다.

active holder만 canonical worktree에서 구현·검증·publication·`execution complete`를 수행한다.
complete는 done과 lease 해제만 기록한다. merge·resource 삭제는 별도 승인 단계다.
