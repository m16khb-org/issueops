# 계획 검토 기록

## 1차: 통과

- 리뷰 모델 anthropic/claude-opus-5-5, effort high. 1차 검토는 수정 요청으로 지적 7건을 냈다. 게이트가 argv 실행과 EXPECT 형식에 맞지 않았고, intent 성공 기준을 덮지 못했으며, 커서 규칙의 도메인 소유를 깨고, 병렬화 기준이 패키지 변수 변경을 놓쳤다. 또 HTTP 취소 전파의 프로토콜 한계, state maintain의 비파괴 계약, Omo 표시 문구 보존 문제가 있었다.
- 2차 delta 검토는 7건이 모두 해소됐음을 확인했고, install/update가 legacy daemon 생존을 확인하지 않고 daemon 디렉터리를 지우는 새 결함 1건을 냈다.
- 3차 delta 검토는 socket dial과 PID 확인 뒤에만 daemon 디렉터리를 지우는 수정을 통과시켰다. unix dial은 즉시 끝나고, PID 재사용이나 zombie 오탐은 삭제를 건너뛰는 안전한 방향으로만 작용한다. 구현 때 legacy PID 파일은 JSON의 pid 필드를 읽어야 한다는 참고가 있다.
