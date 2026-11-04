# 인계받은 세션의 진입 대조

`SKILL.md`의 `## 시작 게이트`가 가리키는 세부 규칙이다. 본문은 Herdr 확인 순서와 `claim` 회복 체인을
소유하고, 이 문서는 인계받은 세션이 진입 전에 대조하는 항목을 소유한다.

진입 전에 인계 결정 기록과 원래 요청의 승인 범위·종료점을 `status`로 대조한다. 나중에 받은 더 넓은
승인이 계획의 종료점보다 우선하며, 계획은 고치지 않고 `decision add --kind scope`로 남긴다.
새 세션은 이전 holder의 release 확인 후 자기 lease로 인수하며, 이미 인계받았으면
재인계하지 않는다. 보류·phase·claim·`--approved`는 구현이나 publication 승인이 아니다.

인계문이 있으면 현재 HEAD, 계획 digest, 인계 자료 digest를 현재 worktree와 대조한다.
stale하거나 누락된 근거는 필요한 범위만 다시 확인한다. 서로 다른 host의 session ID는
이식 가능한 identity가 아니다. 인계 자료와 status를 검증한 뒤 claim 복구 체인을 따른다.

## 인계 준비 중인 child와 worker

인계 준비나 최신 사용자 취소·범위 축소 뒤에는 새 쓰기 작업과 하위 작업 dispatch를
중지한다. 이 스킬이 시작한 child나 worker는 인계 자료에 소유자, 실행 핸들, 입력 리비전,
쓰기 범위, 결과 위치, 읽기 작업인지 쓰기 작업인지의 분류를 남기고, writer와 자손
프로세스가 실제로 종료됐는지 확인한다. 사용자의 최신 취소나 범위 변경이 우선하며,
이전 세션의 callback이나 결과는 source 변경이나 pass 기록의 근거가 될 수 없다.
