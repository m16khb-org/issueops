# 계획 검토 기록

## 1차: 수정 요청

- B6가 isCore를 발화 불가 규칙으로 잘못 분류해, 계획이 유지하겠다고 한 core 재생성 회귀 가드(ownership_forbids_core_package)까지 지우게 되어 있었다.
- B1이 benchmark reliability·consensus usage 줄을 usage.go에 넣으려 했으나, 그 줄의 단일 원본은 internal/contract/cli의 IssueOpsUsageCatalog다.
- B6의 guard 경로 치환은 contract-surface-without-golden finding 조건을 바꿔 guard check 출력이 달라지므로, 출력을 바꾸지 않는다는 약속과 충돌한다.
- ctx 전파 기준 삭제의 근거가 durable intent가 아닌 artifact verification Validate의 읽기 트랜잭션을 덮지 못했다.

## 2차: 수정 요청

- 2라운드 delta 검토에서 usage 단일 원본, guard 경로 보존, artifact 검증 ctx 전파는 해소됐다. 그러나 유지한다던 ownership 가드는 foundation owner의 import에만 적용되어, B6이 지우려는 forbiddenEdges core 절이 전체 그래프의 유일한 core 재도입 차단이었다. 대체 검사 없이 지우면 adapter가 새 core 패키지를 import해도 테스트가 통과한다.

## 3차: 통과

- 3라운드는 claude-opus-5-5에 effort를 xhigh로 한 단계 올려 빈 컨텍스트 세션에서 실행했다. 2라운드에 남은 결함은 B6가 internal/core 재도입을 막는 유일한 전체 그래프 검사를 지우려 한 것이었고, 델타는 architecture 회귀 가드와 guard 접두사를 그대로 두고 G14로 diff가 없음을 강제해 이를 해소했다.
- 응답 계약 golden의 commandguard 경로는 품질 계약 테스트 fake의 커버리지 출력에서 그대로 옮겨진 값이고 파서는 경로 의미를 해석하지 않으므로, 두 곳을 함께 바꾸면 계약 단언이 유지된다. D5의 ctx 전파는 최상위 호출자가 빈 context로 시작해 span 중첩 오류를 새로 만들지 않고, 취소를 존중하는 fake나 실제 저장소로 검증하라는 지시가 기존 fake의 맹점을 피한다.
