# 계획 검토 기록

## 1차: 수정 요청

- 1라운드(claude-opus-5-5, high, claude -p 빈 컨텍스트): direct 모드는 세션을 띄우지 않는다는 전제를 공격했다. execution handoff-cmux가 BuildInteractiveArgv로 native host를 직접 실행하고 resolve는 주입용 정의를 돌려주지 않아 direct 경로 주입이 빠졌다.
- 역할 에이전트 해석 실패를 따라가 보니 Orca adapter가 일반 error를 unknown 호출로 분류해, 설정 오타 하나로 intent가 재시도할 수 없는 pending 상태에 갇힌다.
- 설정 파일이 깨졌을 때 issueops next가 경고로 낮출지 실패할지 정해지지 않았다. resume 봉인, preview와 confirm 일관성, child 판정 근거는 코드와 맞아 공격이 실패했다.

## 2차: 수정 요청

- 2라운드(claude-opus-5-5, high, delta 리뷰): 1라운드의 cmux·Herdr 주입, resolve의 agent_args, 깨진 설정에서 next 동작은 해소됐다.
- resume과 reconcile이 advanceOrca를 거치지 않고 각자 MarkInvoking 뒤 저장된 intent로 terminal을 다시 만들어, 역할 에이전트가 빠지고 설정 오류 때 intent가 unknown으로 남는 문제가 남았다.

## 3차: 수정 요청

- 3라운드는 라운드 규칙에 따라 effort를 high에서 xhigh로 올려 claude-opus-5-5로 실행했다. resume·reconcile의 MarkInvoking 전 주입 위치는 맞지만, resume은 BeginIntent가 pending을 먼저 기록해 실패 뒤 같은 resume 명령이 거부된다.
- reconcile이 쓸 host 출처가 정해지지 않았고, prepare intent는 dispatch 전까지 Orca binding이 없어 sealed intent의 Probe.Host를 써야 한다. 주입 인자 추가가 기존 terminal 탐색과 delivery wrapper를 깨지 않는다는 점은 확인됐다.

## 4차: 통과

- 4라운드(claude-opus-5-5, xhigh, delta 리뷰): resume은 BeginIntent 앞에서 주입 인자를 계산하므로 실패해도 pending intent가 생기지 않고 같은 resume 명령으로 다시 시도할 수 있다.
- reconcile은 정규화한 sealed intent의 Probe에서 host를 가져오고 Orca terminal 생성도 같은 Probe.Host를 써서, binding이 없는 prepare intent에도 맞는 역할 에이전트가 주입된다.
- resume의 terminal 조건은 기존 binding 재사용 plan에서도 참이므로 주입 인자 계산은 기존 binding의 early return 뒤에 둬야 한다. G5 테스트 범위에 outbound issueopslease 패키지를 포함한다는 참고와 함께 구현 단계로 넘긴다.
