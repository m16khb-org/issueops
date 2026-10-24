---
name: issueops-review
description: 'Use when issueops-plan or issueops-verify needs an independent review of a cycle architecture document, plan, or implementation diff, or for "아키텍처 검토", "계획 검토", "구현 리뷰", "devil''s advocate" in an IssueOps cycle.'
---

# IssueOps Architecture, Plan, and Implementation Review

이 스킬의 일은 **적대 리뷰 한 번을 실제로 실행하고 그 판정을 원장에 기록하는
것**이다. 계획 단계(아키텍처 문서와 계획)와 검증 단계(구현 diff)가 같은 규칙을 쓰도록 두 곳에서 이 스킬을
호출한다. 대상 파일을 고치는 것은 호출한 단계의 일이다.

- 계획 단계: [`issueops-plan`](../issueops-plan/SKILL.md)
- 검증 단계: [`issueops-verify`](../issueops-verify/SKILL.md)
- 리뷰 렌즈 원문: [`architecture-review`](../architecture-review/SKILL.md)
- 원격 반영 프로토콜: [`issueops-remote-write`](../issueops-remote-write/SKILL.md)

## 입력

`--target architecture`, `--target plan`, `--target diff` 세 가지다. 대상을 정하고 리뷰어에게 줄 자료를
모은다.
표(대상별 리뷰 대상·함께 주는 자료), 색인 문서 열기 규칙, `CONSTITUTION` 발췌 규칙은
[입력 자료 준비](references/input-materials.md)를 리뷰어를 띄우기 전에 읽는다.
plan과 diff 모두 리뷰 전에 `.issueops/ADR.md`, `.issueops/CAUTIONS.md`, `.issueops/ARCHITECTURE.md`
세 색인을 직접 열어 관련 항목과 그 모듈 파일을 연다. grep으로 한 줄만 본 것은 연 것이 아니다.

리뷰를 실행하기 직전에 exact lifecycle ID로 다음 명령을 읽는다. 준비 자료의
reviewer 메타데이터는 준비 당시 기본값이며 현재 리뷰 설정을 대신하지 않는다.

```bash
issueops next --id "$ISSUEOPS_ID" --json
# .review.model, .review.effort, .review.tier, .review.lenses
```

이 값은 리뷰 역할(implement 전은 `plan-review`, 이후는 `diff-review`)을 `issueops model`
설정(local > global > 내장 기본값)으로 해석한 model·effort와 현재 변경 집합 티어다. 스킬 본문에
모델 이름 표를 복사하지 않는다. 조회에 실패하거나 review의 model/effort/tier/lenses가 누락되면
진행하지 않고 조회 실패나 누락 필드를 보고한다. `review.model`이 비어 있고 `warnings`에
`agent model settings are invalid`가 있으면 그 파일 경로를 보고하고 멈춘다. 경고 없이 비어
있으면 이 호스트의 기본값이 없으므로 어떤 모델로 리뷰할지 사용자에게 묻는다.
사용자가 reviewer 모델이나 effort를 명시했다면 해당 값에만 그 지시를 우선하고,
지정하지 않은 값은 현재 출력을 따른다. owner-model/owner-effort override는 구현자
선택이며 reviewer 지정으로 해석하지 않는다.

`--target diff`는 정리와 문서 반영이 끝나 봉인된 diff에 실행한다. 검증과 리뷰의
동시 실행은 `issueops-verify`를 따르며, 모든 필수 결과를 확인한 뒤 판정을 기록한다.
판정이 fingerprint에 묶이므로, 리뷰 뒤 파일을 고치면 그 판정은 무효가 된다.

## 실행

현재 호스트의 delegation 도구로 architecture-review를 **빈 컨텍스트의 새 세션**에 띄운다.
띄우기는 이 세션이 지금 하는 도구 호출이다. 리뷰어 프롬프트를 파일로 써 두고 끝내지 않는다.
기록할 판정과 finding은 서브에이전트가 돌려준 출력에서 옮긴다. 서브에이전트를 백그라운드로 띄웠으면 결과가
도착할 때까지 기다린다. 시작됐다는 응답(실행 ID, launch 안내)은 판정이 아니다. 저자 세션이 먼저 읽고 내린
판단은 판정이 아니므로 기록하지 않는다. 서브에이전트를 띄우지 못했으면 판정 없이 그 사실과
이유를 보고한다. `--reviewer-context subagent`는 서브에이전트가 실제로 돌려준 판정에만 쓴다.
사용자가 서브에이전트 없이 inline 리뷰를 명시적으로 지시했으면 이 세션이 같은 출력 계약으로
검토하고 `--reviewer-context inline`으로 기록한다. reviewer 모델과 effort는 이 세션의 값을 적고,
독립 리뷰가 아니라는 사실을 보고에 남긴다.
필수 수정의 기준은 [`architecture-review`의 Blocking threshold](../architecture-review/SKILL.md#blocking-threshold)를
따른다. `architecture`는 아키텍처 문서의 결정과 계약을, `plan`은 계획을, `diff`는 구현 동작과 영향받은 계약을 확인한다.
리뷰가 지적을 냈다는 이유만으로 수정하지 않고, 발생 조건·위반한 요구사항·근거가
맞는지 확인한 뒤 필요한 수정만 한다. 반증된 지적은 그 증거를 재리뷰에 전달한다.

저자 세션이 architecture-review의 게이트를 직접 밟아 보는 것은 리뷰가 아니다. 문서나 계획을 쓴
에이전트는 매몰 비용을 지고 있고 자기 합리화를 기억하며 자기 설계를 무의식적으로
변호한다. 독립성이 기제 전부다. 계획에 아무 투자도 하지 않은 새 컨텍스트만이 저자가
못 보는 결함을 본다.
프롬프트에 [리뷰어 프롬프트 구성과 렌즈](references/reviewer-prompt.md)의 항목을 전부 넣는다.
서브에이전트는 빈 컨텍스트에서 시작하므로 여기 없는 것은 리뷰어에게 존재하지 않는다. 핵심 규칙:

- 출력 계약은 판정(`pass|revise|stop`)과 필수 결함별 위치·발생 조건·위반한 계약·근거·최소 수정 또는
  확인 방법이다. 선택적 개선 목록과 대안 설계는 요구하지 않는다.
- 위반한 계약 칸은 문서 규칙 원문(경로:줄)이나 대상이 약속한 동작이 실제로 깨지는 입력·경로로 채운다.
  채우지 못한 지적은 비차단 메모다. 판정 전에 인용 줄을 다시 열어 확인한다.
- 명령으로 판별할 수 있는 지적에는 `CHECK: <read-only 명령> | EXPECT: <출력에 있어야 할 문자열>`을
  요구한다. 필수 계약의 증거가 빠졌으면 **필수 검증 공백**으로 남기고 `revise` 또는 `stop`으로 판정한다.
- 대상에 맞는 렌즈를 적용한다. `diff`에는 `next.review.tier`와 `lenses`를 전달하고, 결함·회귀 수정이면
  근본 원인 렌즈를 더한다. 코드베이스 존중 렌즈(재사용·성능·하위 호환·side effect)도 같은 문서에 있다.

이름·스타일·선택적 리팩터링·가상 규모를 이유로 수정이나 추가 테스트를 요구하지 않는다.
영향받은 필수 동작의 증거가 부족하면 그 가정을 판별하는 최소 확인을 요청한다.
재현된 실패가 없다는 이유로 확인되지 않은 필수 동작을 통과시키지는 않는다.

렌즈에 걸린 항목도 [Blocking threshold](../architecture-review/SKILL.md#blocking-threshold)를 넘어야
필수 결함이다. 작은 중복의 기준은 [리뷰어 프롬프트 구성과 렌즈](references/reviewer-prompt.md)의 4번 출력 계약을 따른다.

## 지적을 실행한다

리뷰어가 낸 CHECK는 호출자가 worktree에서 그대로 실행한다. 실행 결과는 다음 라운드의
**입력**이지 판정이 아니다. `verify-work -- <CHECK>`는 읽기 전용 allowlist 명령만 받아
테스트·스크립트 CHECK를 거부하므로 쓰지 않는다.

- `--target architecture`, `--target plan`, `--target diff` 모두 이 경로로 실행한다. 계획 단계에는 아직 게이트
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

기록 명령(target별 `architecture review`·`devils-advocate review`·`implementation-review record`),
감사 필드, finding 작성 규칙, stale 규칙, 이슈 반영은
[기록 명령과 이슈 반영](references/record-and-issue.md)을 따른다. 핵심: `--reviewer-model`·`--reviewer-effort`에는
실제 리뷰 실행에 넘긴 값을 넣는다. inline 리뷰는 별도 실행이 없으므로 현재 세션의 모델과 effort를 적고,
effort를 알 수 없으면 `unknown`이라고 적는다. `next.review`의 권장 모델로 바꿔 적지 않는다. 승인은 문서 sha256에 묶이며(리뷰 뒤 수정하면 stale), `reviewer_*`는 감사 필드이지
증명이 아니므로 사실대로 적는 것은 실행자의 책임이다. 계획 판정은 `issueops remote reflect-devils-advocate --confirm`으로
반영하고 `stop` 판정일 때 특히 중요하다.

## 루프 규칙

- 첫 라운드는 대상 전체를 검토한다. 수정 뒤의 라운드는 **delta 리뷰**다. 대상 전체를
  다시 읽히지 않고 직전 지적, 각 CHECK의 실행 결과, 대상의 delta, 영향받은 계약만
  새 컨텍스트 서브에이전트에 넘긴다. 판정과 기록할 finding은 그 서브에이전트가 정하고
  호출자는 받은 verdict를 그대로 기록한다. CHECK가 전부 통과했다는 사실은 delta 리뷰의
  입력이지 호출자가 `pass`를 정할 근거가 아니다.
  호출자는 다음 규칙을 delta 리뷰 프롬프트에 넣는다. delta 리뷰어는 직전 지적마다 해결됨 또는 남음만
  판정하고, 남음이면 남은 위치와 이유를 적는다. 반증 자료로 철회된 지적은 해결됨으로 적고 반증 근거를
  붙인다. 새 필수 결함은 delta와 영향받은 계약에서만 찾는다. 직전 라운드에 나온 지적을 다른 말로 다시
  내지 않는다.
- delta 리뷰를 띄우기 **전에** 호출자가 수정한 대상을 직접 점검한다(옛 값 검색, 새 단언 사실 확인,
  `issueops gates lint` 통과). 세부는 [루프 세부 규칙](references/loop-details.md)을 읽고, 세 점검의
  결과를 delta 리뷰 입력에 함께 넣는다.
- 구조나 범위가 바뀌면 전체 리뷰를 다시 띄운다. 필수 검증 공백은 그 확인 결과와
  영향받은 계약을 delta 리뷰에 포함한다. CHECK 형식이 없다는 이유만으로 전체를 반복하지 않는다.
- 같은 대상의 수정·재리뷰는 최대 5라운드다. 3라운드부터 5라운드까지는 상향된 리뷰 모델로
  띄운다. model과 effort는 다음 명령의 결과를 그대로 쓴다. `review-escalate` 설정이 있으면 그
  값이, 없으면 원래 리뷰 모델에 effort 한 단계 상향이 나온다. architecture와 plan 대상이면 `--role plan-review`다.

  ```bash
  issueops model resolve --host "$HOST" --role diff-review --tier "$TIER" --round "$ROUND" --json
  # .model, .effort, .argv
  ```

  사용자가 reviewer 모델을 이름으로 지정했으면 그 값을 `--model`로 함께 넘긴다. 실제로 넘긴 두
  값을 `--reviewer-model`·`--reviewer-effort`(diff) 또는 finding 첫 줄(plan)에 적는다. 그 안에
  통과하지 못하면 남은 결함과 시도한 수정을 보고한다.
  상향 모델 해석, 빈 컨텍스트 세션 `.argv` 실행, Fable 5 금지는 [루프 세부 규칙](references/loop-details.md)을 따른다.
- 같은 plan phase의 비-waived `revise`는 다섯 번까지다. 여섯 번째는 CLI가
  `revise round cap reached`로 거부한다. stop 판정 뒤의 `regress` 재계획도 사이클당 다섯 번까지다. 그때의 탈출은 `stop`을 기록하고
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

## 나쁜 예

[나쁜 예](references/bad-examples.md)를 리뷰 전에 훑는다. 핵심: 저자 세션 인라인 리뷰를 `subagent`로 기록하지
않고, 리뷰 없이 `pass`를 기록하지 않으며, CHECK 통과만으로 `pass`를 정하지 않고, `revise`를 `--waive`로 닫지 않는다.

## 검증

- `issueops status --id "$ISSUEOPS_ID" --json`의 `devils_advocate_review`
  또는 `implementation_review`에 이번 판정이 있고 digest·fingerprint가 현재 대상과
  같은지 확인한다.
- `issueops next --id "$ISSUEOPS_ID" --json`의 `missing`에
  `devils_advocate_review*`·`implementation_review*`가 남아 있지 않은지 확인한다.
- 남아 있으면 그 키가 곧 다음 명령이다. 추측하지 말고 `next_command`를 실행한다.
