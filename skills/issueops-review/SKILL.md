---
name: issueops-review
description: Use when issueops-plan or issueops-verify needs an independent review of a cycle plan or implementation diff, or when the user asks for "계획 검토", "구현 리뷰", "design-review", or "devil's advocate" within an IssueOps cycle.
---

# IssueOps Plan and Implementation Review

이 스킬의 일은 **적대 리뷰 한 번을 실제로 실행하고 그 판정을 원장에 기록하는
것**이다. 계획 단계와 검증 단계가 같은 규칙을 쓰도록 두 곳에서 이 스킬을
호출한다. 대상 파일을 고치는 것은 호출한 단계의 일이다.

- 계획 단계: [`issueops-plan`](../issueops-plan/SKILL.md)
- 검증 단계: [`issueops-verify`](../issueops-verify/SKILL.md)
- 리뷰 렌즈 원문: [`design-review`](../design-review/SKILL.md)
- 원격 반영 프로토콜: [`issueops-remote-write`](../issueops-remote-write/SKILL.md)

## 입력

`--target plan`과 `--target diff` 두 가지다. 대상을 정하고 리뷰어에게 줄 자료를
모은다.

| target | 리뷰 대상 | 함께 주는 자료 |
|---|---|---|
| `plan` | `status --json`의 `plan_path` 파일 전체(링크 전이면 staged plan artifact) | 이슈 본문, intent contract의 성공 기준, design review 본문 |
| `diff` | `git -C "$WORKTREE" diff "$BASE_SHA"` 전체 | plan 파일 전체, 봉인 intent 문서(`<artifact_dir>/intent.md`, 없으면 `status --json`의 `.intent`), 검증 명령과 결과, 변경한 프로젝트 문서 |

리뷰어 모델과 effort는 다음 명령이 돌려준다.

```bash
issueops next --id "$ISSUEOPS_ID" --json
# .review.model, .review.effort
```

이 값은 코드가 소유하는 host별 planner 기본값이다. 스킬 본문에 모델 이름 표를
복사하지 않는다 — 복사한 순간 그 표는 코드보다 먼저 낡는다. `review.model`이 비어
있으면 이 호스트의 기본값이 정의돼 있지 않다는 뜻이므로, 진행하지 말고 어떤 모델로
리뷰할지 사용자에게 묻는다.

`--target diff`는 정리와 문서 반영이 끝나 봉인된 diff에 실행한다. 검증과 리뷰의
동시 실행은 `issueops-verify`를 따르며, 모든 필수 결과를 확인한 뒤 판정을 기록한다.
판정이 fingerprint에 묶이므로, 리뷰 뒤 파일을 고치면 그 판정은 무효가 된다.

## 실행

현재 호스트의 delegation 도구로 design-review를 **빈 컨텍스트의 새 세션**에 띄운다.
필수 수정의 기준은 [`design-review`의 Blocking threshold](../design-review/SKILL.md#blocking-threshold)를
따른다. `plan`은 계획을, `diff`는 구현 동작과 영향받은 계약을 확인한다.
리뷰가 지적을 냈다는 이유만으로 수정하지 않고, 발생 조건·위반한 요구사항·근거가
맞는지 확인한 뒤 필요한 수정만 한다. 반증된 지적은 그 증거를 재리뷰에 전달한다.

저자 세션이 design-review의 게이트를 직접 밟아 보는 것은 리뷰가 아니다. 계획을 쓴
에이전트는 매몰 비용을 지고 있고 자기 합리화를 기억하며 자기 설계를 무의식적으로
변호한다. 독립성이 기제 전부다. 계획에 아무 투자도 하지 않은 새 컨텍스트만이 저자가
못 보는 결함을 본다.

프롬프트에 다음을 전부 넣는다. 서브에이전트는 빈 컨텍스트에서 시작하므로 여기 없는
것은 리뷰어에게 존재하지 않는다.

1. 대상 전문(위 표의 리뷰 대상과 함께 주는 자료).
2. 성공 기준과 범위 경계. `diff` 대상이면 intent 문서의 성공 기준·비목표 대비 diff가 무엇을 덮고 무엇을
   벗어나는지도 판정하게 한다.
3. 관련 ADR과 CAUTIONS 항목의 경로와 제목.
4. 출력 계약: 판정(`pass|revise|stop`), 필수 결함별 위치·발생 조건·위반한 계약·근거와
   최소 수정 또는 확인 방법. 발견한 필수 결함은 한 번에 전달한다. 결함이 없으면
   없다고 쓰고 확인한 계약과 근거를 남긴다. 선택적 개선 목록과 대안 설계는 요구하지 않는다.
   명령으로 판별할 수 있는 지적에는
   `CHECK: <read-only 명령> | EXPECT: <출력에 있어야 할 문자열>`을 함께 요구한다.
   한 줄 명령으로 표현할 수 없으면 확인할 가정·절차·통과 조건을 적는다. 필수 계약의
   증거가 빠졌으면 **필수 검증 공백**으로 남기고 `revise` 또는 `stop`으로 판정한다.
   호출자가 확인한 결과를 다음 라운드에 전달하며, CHECK 형식의 유무로 차단 여부를
   바꾸지 않는다([`design-review`의 Blocking threshold](../design-review/SKILL.md#blocking-threshold)).
5. 대상에 맞는 렌즈. `plan`에는 design-review의 계획 렌즈와 아래 코드베이스 존중
   렌즈를 적용한다. 구현 전 `next.review.tier`는 변경 경로를 조회하지 않은 `default`이므로
   계획의 위험 분류로 해석하지 않는다. 계획에 명시한 인증·스키마 등 필수 계약을 확인한다.
   `diff`에는 `next.review.tier`와 `lenses`를 전달하고 선택된 렌즈만 적용한다.
   `docs-only`의 코드베이스 존중 렌즈는 `side-effect` 하나다. frontend 추가 렌즈는
   호출한 검증 단계의 규칙을 따른다.

이름·스타일·선택적 리팩터링·가상 규모를 이유로 수정이나 추가 테스트를 요구하지 않는다.
영향받은 필수 동작의 증거가 부족하면 그 가정을 판별하는 최소 확인을 요청한다.
재현된 실패가 없다는 이유로 확인되지 않은 필수 동작을 통과시키지는 않는다.

코드베이스 존중 렌즈는 다음을 묻는다.

- **재사용**: 이미 있는 구현으로 됐을 일을 새로 만들지 않았는가. 같은 판정을 두 곳이
  소유하게 되지 않았는가.
- **성능**: 이 변경이 자주 불리는 경로에 관측·조회·할당을 늘리지 않는가. 같은 관측을
  두 번 하지 않는가.
- **하위 호환**: 계약 표면(포트 인터페이스, CLI 플래그, JSON 필드, 온디스크 형식)이
  기존 호출자를 깨지 않는가. 기본값이 종전 동작을 유지하는가.
- **side effect**: 파일·원격·durable state에 남는 변화가 문서화된 것과 일치하는가.
  실패했을 때 남는 상태가 사람이 이어받을 수 있는 모양인가.

## 지적을 실행한다

리뷰어가 낸 CHECK는 호출자가 실행한다. 실행 결과는 다음 라운드의 **입력**이지
판정이 아니다.

```bash
issueops verify-work --json -- <CHECK>
```

- `--target plan`과 `--target diff` 모두 이 경로로 실행한다. 계획 단계에는 아직 게이트
  원장 파일이 없다(원장은 4단계 진입의 `gates init`이 만든다). 그러니 여기서 `gates`
  명령을 부르지 않는다.
- plan 리뷰에서 살아남은 CHECK/EXPECT는 4단계 진입의 그 단일 `gates init` spec에
  `G(n+1)..`로 얹어 원장의 일부가 되게 한다.
- diff 리뷰의 CHECK는 원장에 넣지 않는다. 검증 단계에서 파일을 고치면 봉인이 바뀐다.
- 각 CHECK의 명령·종료 코드·EXPECT 일치 여부를 그 결함 옆에 적어 다음 라운드에 넘긴다.

## 기록

리뷰가 **끝난 뒤에만** 호출자가 기록한다. 기록이 리뷰를 대신하지 않는다.
`diff` 리뷰를 검증 배터리·스키마 확인·QA와 병렬로 수행했다면 모든 필수 결과를 모으고
현재 fingerprint가 시작 때와 같은지 확인한 뒤 기록한다. 검증 실패나 미확인 필수 계약이
있으면 먼저 도착한 `pass`를 기록하지 않는다. 실패 증거는 호출한 단계에 전달한다.

```bash
# --target plan
issueops devils-advocate review --id "$ISSUEOPS_ID" \
  --verdict pass --reviewer-context subagent \
  --finding "<무엇을 공격했고 왜 살아남았는가>" $RECORD_ACTOR_FLAGS --json

# --target diff
issueops implementation-review record --id "$ISSUEOPS_ID" \
  --verdict pass --finding "<finding>" --evidence "<evidence>" \
  --reviewer-host "$HOST" --reviewer-model "$REVIEWER_MODEL" --reviewer-effort "$REVIEWER_EFFORT" \
  $RECORD_ACTOR_FLAGS --json
```

finding은 이슈의 `## 계획 검토` 구간과 `.issueops/issues/<n>/plan-review.md`로 팀에 보인다.
독자가 읽는 한국어 완성 문장으로 쓴다: 무엇을 공격했고 결과가 어땠는지. 해시, 커밋 SHA
전문, 로컬 절대 경로는 쓰지 않는다(렌더러가 가리지만 문장이 깨진다). 리뷰가 이슈
본문의 사실이 틀렸다고 판정하면 호출 단계가 `feedback add --classification
contract_change`로 기록한다([`issueops-plan`](../issueops-plan/SKILL.md)의 검토 루프).

`reviewer_context`와 `reviewer_*`는 감사 필드이지 게이트 조건이 아니다. 하네스는
모델의 자기신고를 검증할 수 없으므로 verdict와 finding·evidence의 실질만 게이트한다.
그래서 이 필드를 사실대로 적는 것은 도구가 아니라 실행자의 책임이다.

## 루프 규칙

- 첫 라운드는 대상 전체를 검토한다. 수정 뒤의 라운드는 **delta 리뷰**다. 대상 전체를
  다시 읽히지 않고 직전 지적, 각 CHECK의 실행 결과, 대상의 delta, 영향받은 계약만
  새 컨텍스트 서브에이전트에 넘긴다. 판정과 기록할 finding은 그 서브에이전트가 정하고
  호출자는 받은 verdict를 그대로 기록한다. CHECK가 전부 통과했다는 사실은 delta 리뷰의
  입력이지 호출자가 `pass`를 정할 근거가 아니다.
- 구조나 범위가 바뀌면 전체 리뷰를 다시 띄운다. 필수 검증 공백은 그 확인 결과와
  영향받은 계약을 delta 리뷰에 포함한다. CHECK 형식이 없다는 이유만으로 전체를 반복하지 않는다.
- 같은 대상의 수정·재리뷰는 최대 3라운드다. 3라운드는 `next.review.model`과 다른 모델
  또는 한 단계 높은 effort로 띄우고, 그 사실을 `--reviewer-model`·`--reviewer-effort`
  (diff) 또는 finding 첫 줄(plan)에 적는다. 그 안에 통과하지 못하면 남은 결함과 시도한
  수정을 보고한다.
  - Claude에서 "다른 모델"은 사용자가 이름으로 지정한 모델만 쓴다. Fable 5는 명시적
    수동 지정 전용이므로(`internal/contract/issueopspreparation/prepare.go`) 3라운드용으로
    고르지 않는다. codex와 omo는 이 항목의 적용을 받지 않는다.
  - Claude는 3라운드에도 `next.review.model`을 쓰고, effort는 `next.review.effort`에서
    한 단계 올린다. claude CLI의 단계는 `low`→`medium`→`high`→`xhigh`→`max`다.
    서브에이전트 도구는 effort를 받지 않으므로, 프롬프트를 표준 입력으로 넘겨
    `claude -p --model "$REVIEW_MODEL" --effort "$ROUND3_EFFORT" --allowedTools
    "Bash Read Grep Glob Skill"`로 빈 컨텍스트 세션을 띄운다. 출력 파일은 ignored 영역
    `.issueops/issues/<n>/review/`나 워크트리 밖에 둔다. 워크트리 안 미추적 파일은
    fingerprint에 들어가 봉인을 깬다.
    `--reviewer-model`·`--reviewer-effort`와 finding 첫 줄에는 실제로 넘긴 두 값을 적는다.
- 같은 plan phase의 비-waived `revise`는 세 번까지다. 네 번째는 CLI가
  `revise round cap reached`로 거부한다. 그때의 탈출은 `stop`을 기록하고
  `issueops remote reflect-devils-advocate --confirm`으로 반영한 뒤 `regress`로
  재계획하거나, 근거를 적은 `--waive --waiver-rationale`로 넘어가는 것이다. `revise`
  상태에서 `regress`를 직접 부르면 거부된다.
- `revise`면 호출한 단계가 근거를 확인해 필요한 결함을 고치고 이 스킬을 다시 실행한다.
  반증된 지적만 남았으면 대상을 불필요하게 바꾸지 않고 반증 자료로 새 판정을 받는다.
- `stop`이면 `--target plan`은 호출자가 `issueops regress --id
  "$ISSUEOPS_ID" --reason "<TEXT>"`로 grill까지 되돌려 재조사·재계획한다.
  `--target diff`는 publication을 멈춘다. 승인 범위 안에서 해소할 수 있는 결함은
  구현 단계로 돌아가 수정하고 다시 리뷰한다. 범위·권한·요구사항 결정이 필요할 때만
  사용자에게 묻는다. stop 판정을 pass로 바꾸거나 우회하지 않는다.
- 현재 digest·fingerprint에 유효한 `pass`가 있고 새 결함 증거나 계약 변경이 없으면
  재사용한다. 불안감이나 선택적 개선은 추가 리뷰 사유가 아니다.
- `pass`는 finding이 하나 이상 있어야 기록된다. 이 finding은 확인한 계약과 안전한 이유를
  적는 증거이며, 결함을 하나 이상 만들어 내라는 뜻이 아니다.
- `--waive`는 override이며 `--waiver-rationale`이 필수다. "지적을 반영했다"는 뜻으로
  쓰지 않는다. 반영했으면 다시 리뷰해서 새 판정을 받는다.
- 판정은 plan sha256(`reviewed_plan_digest`) 또는 change fingerprint
  (`reviewed_fingerprint`)에 묶인다. 판정 뒤 대상을 고치면
  `devils_advocate_review_stale`·`implementation_review_stale`이 되어 implement 진입과
  publication이 막히므로 다시 실행해 새 판정을 기록한다.

## 이슈 반영

계획 판정을 팀이 보게 하려면 [`issueops-remote-write`](../issueops-remote-write/SKILL.md)의
절차로 다음을 실행한다.

```bash
issueops remote reflect-devils-advocate --id "$ISSUEOPS_ID" --confirm --json
```

이슈에는 라운드 흐름 한 줄("1차 수정 요청(지적 3건) → 계획 수정 → 2차 통과")이
보이고, `stop` 판정일 때만 중단 이유가 목록으로 붙는다. 지적 원문은 record와
`plan-review.md`에 있다. `stop` 판정은 반영이 특히 중요하다. 사이클이 뒤로 돌아간
이유가 이슈에 남지 않으면 팀이 보는 진행 상태와 실제가 어긋난다.

## 나쁜 예

- 저자 세션이 인라인으로 게이트를 밟고 `--reviewer-context subagent`로 기록한다.
  기록은 통과하지만 리뷰는 없었다.
- 리뷰를 실행하지 않고 `--verdict pass`를 기록한다. 게이트 연극이다.
- CHECK가 전부 통과했다는 이유로 호출자가 `pass`를 기록한다. 판정 없는 기록이다.
- `revise` 판정을 고치는 대신 `--waive`로 닫는다.
- 판정 뒤 계획이나 코드를 고치고 재검토를 생략한다. stale 판정으로 다음 단계에서
  막히고, 막히지 않았다면 검토되지 않은 변경이 게시된 것이다.
- `reviewer_model`이 기록됐으니 planner급 모델이 돌았다고 믿는다. 그 필드는 감사
  기록이지 증명이 아니다.
- `--target diff` 리뷰에 plan을 주지 않는다. 무엇을 하기로 했는지 모르는 리뷰어는
  구현이 계획에서 벗어났는지 판정할 수 없다.
- 3라운드에서 '다른 모델'로 Fable 5를 고른다. Fable 5는 사용자가 이름으로 지정할 때만 쓴다.

## 검증

- `issueops status --id "$ISSUEOPS_ID" --json`의 `devils_advocate_review`
  또는 `implementation_review`에 이번 판정이 있고 digest·fingerprint가 현재 대상과
  같은지 확인한다.
- `issueops next --id "$ISSUEOPS_ID" --json`의 `missing`에
  `devils_advocate_review*`·`implementation_review*`가 남아 있지 않은지 확인한다.
- 남아 있으면 그 키가 곧 다음 명령이다. 추측하지 말고 `next_command`를 실행한다.
