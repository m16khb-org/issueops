# 기록 명령과 이슈 반영

`SKILL.md`의 `## 기록`에서 호출한다.

리뷰가 **끝난 뒤에만** 호출자가 기록한다. 기록이 리뷰를 대신하지 않는다.
`diff` 리뷰를 검증 배터리·스키마 확인·QA와 병렬로 수행했다면 모든 필수 결과를 모으고
현재 fingerprint가 시작 때와 같은지 확인한 뒤 기록한다. 검증 실패나 미확인 필수 계약이
있으면 먼저 도착한 `pass`를 기록하지 않는다. 실패 증거는 호출한 단계에 전달한다.

```bash
# --target architecture (pass일 때만 --approved)
issueops architecture review --id "$ISSUEOPS_ID" \
  --problem-summary "<문제>" --proposed-architecture "<아키텍처 결정>" --refactor-plan "<리팩터 계획>" \
  --alternative "<기각한 대안>" --risk "<위험>" --verification "아키텍처 검토로 대안과 위험을 확인했다" \
  --reviewer-context subagent --approved $RECORD_ACTOR_FLAGS --json

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

`--reviewer-model`과 `--reviewer-effort`에는 실제 리뷰 실행에 넘긴 값을 shell-quote해
넣는다. 생성 owner command의 `<REVIEWER_MODEL>`·`<REVIEWER_EFFORT>`도 같은 값으로
채운다. 기본 티어 선택과 아래 3라운드부터의 상승은 구분하고, 상승 이유는 finding에 남긴다.
준비 자료의 고정값이나 상승 전 값을 감사 필드에 기록하지 않는다.

finding은 이슈의 `## 계획 검토` 구간과 `.issueops/issues/<n>/plan-review.md`로 팀에 보인다.
독자가 읽는 한국어 완성 문장으로 쓴다: 무엇을 공격했고 결과가 어땠는지. 해시, 커밋 SHA
전문, 로컬 절대 경로는 쓰지 않는다(렌더러가 가리지만 문장이 깨진다). 리뷰가 이슈
본문의 사실이 틀렸다고 판정하면 호출 단계가 `feedback add --classification
contract_change`로 기록한다([`issueops-plan`](../../issueops-plan/SKILL.md)의 검토 루프).

`architecture review`의 승인은 스테이징한 아키텍처 문서의 sha256에 묶인다. 리뷰 뒤 문서를 고치면
`architecture_review_stale`이므로 다시 스테이징하고 새 리뷰를 받는다. `revise`면 문서를 고쳐 다시
리뷰하고(같은 5라운드 상한), `stop`이면 승인을 기록하지 않고 이슈 본문의 사실 변경을
`feedback add --classification contract_change`로 남긴 뒤 이슈를 동기화한다.

`reviewer_context`와 `reviewer_*`는 감사 필드이지 게이트 조건이 아니다. 하네스는
모델의 자기신고를 검증할 수 없으므로 verdict와 finding·evidence의 실질만 게이트한다.
그래서 이 필드를 사실대로 적는 것은 도구가 아니라 실행자의 책임이다.

## 이슈 반영

계획 판정을 팀이 보게 하려면 [`issueops-remote-write`](../../issueops-remote-write/SKILL.md)의
절차로 다음을 실행한다.

```bash
issueops remote reflect-devils-advocate --id "$ISSUEOPS_ID" --confirm --json
```

이슈에는 라운드 흐름 한 줄("1차 수정 요청(지적 3건) → 계획 수정 → 2차 통과")이
보이고, `stop` 판정일 때만 중단 이유가 목록으로 붙는다. 지적 원문은 record와
`plan-review.md`에 있다. `stop` 판정은 반영이 특히 중요하다. 사이클이 뒤로 돌아간
이유가 이슈에 남지 않으면 팀이 보는 진행 상태와 실제가 어긋난다.
