# 계획 검토 기록

## 1차: 수정 요청

- S1이 쓰려던 systemd-analyze --user unit-paths는 공식 문서상 실행 중인 manager와 통신하지 않고 호출 프로세스의 환경으로 경로를 계산하므로, HOME을 바꿔 설치하면 불일치를 감지하지 못하고 fake Runner 테스트도 이를 가린다. UnitPath 속성만 쓰면 없는 디렉터리가 빠져 같은 HOME의 첫 설치에서 거짓 오류가 난다.
- S3의 git grep 제외 규칙은 magic 없는 pathspec이 디렉터리 경로까지 맞춰, 이름에 lock이 든 디렉터리의 정의를 놓친다. 쓰기 전 차단 순서, supervisor 인터페이스 확장, source 줄 소비처, 테스트 결정성은 문제가 없었다.

## 2차: 통과

- 2라운드 delta 검토에서 1라운드 결함 두 개가 해소됐다. supervisor 사전 감지는 실행 중인 user manager의 환경과 UnitPath를 함께 비교해, HOME이 다른 설치는 쓰기 전에 명시 오류로 멈추고 같은 HOME의 첫 설치는 막지 않는다.
- CI 회귀 단계는 빌드를 건너뛰는 설치에서도 begin 단계가 credential·unit을 쓰기 전에 새 검사를 거치고 임시 HOME에 아무것도 남기지 않는다. git grep 제외 패턴은 glob magic으로 고쳐졌다. 성능 영향 문구와 G4 테스트 개수는 비차단 불일치로 구현 시 맞춘다.
