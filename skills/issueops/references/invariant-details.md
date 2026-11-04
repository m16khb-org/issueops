# 공통 불변식 세부 규칙

`SKILL.md`의 `## 공통 불변식` (b)·(c)가 가리키는 세부 규칙이다. 본문은 각 불변식의 결론을 소유하고,
이 문서는 actor 플래그를 쓰는 방법과 lease 전후의 whoami 사용을 소유한다.

## (b) actor 플래그

`next_command`는 호출 세션의 플래그를 채워서 돌려준다. `next_command_kind`가 `exact`면 그대로 실행한다.
채우지 못한 자리표시자와 스킬 예시의 변수는 `issueops execution whoami --json` 값으로 바꾼다:
`$RECORD_ACTOR_FLAGS` = `record_actor_flags`(문자열), `$ACTOR_FLAGS` = `claim_actor_flags[0]`(배열의 첫 항목). 손으로 조립하지 않는다.
두 값은 셸 따옴표가 붙은 문자열이다. 따옴표 없는 `$VAR`로 넘기면 셸이나 CLI가 나눠 읽는다. `${=VAR}`는 쓰지 않는다.

## (c) lease fencing

durable mutation(phase 전이, record 기록, artifact stage) 전마다
exact lifecycle ID·generation·native actor·canonical cwd를 현재 record와 대조한다.
record 기록은 `RECORD_ACTOR_FLAGS`, lease 전이와 publication은 `ACTOR_FLAGS`를 쓴다. 불일치는 stop이다. 사용자의
"그 세션은 내가 껐어"는 quiescence 증거가 아니다. 모호한 결과는 재실행하지 않고 reconcile한다.
lease 전의 준비 기록은 source checkout의 native actor로, 구현은 active holder의 canonical cwd로 한다.
lease를 잡은 뒤 whoami는 `cd <worktree> && issueops execution whoami --json`으로 다시 받는다(source checkout 값은 `--cwd`가 달라 거부된다).
