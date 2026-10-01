# 요청자 의도 계약

- lifecycle: io-59a5d211f12e
- issue: https://github.com/m16khb-org/issueops/issues/544
- intent_class: standard

## 원문 요청
1,2,3을 전부 $issueops 로 6.1 sol 병렬인계로 개선을 진행해주고 머지하고 $issueops-cleanup 후 $io-update 까지 해줘

## 해석
개선점 3: 문서 변경의 필수 완료 검증과 선택적 운영 명령 예시를 분리하고 self-verify가 수행한 검사의 중복 실행과 불필요한 사용자 홈 설치를 제거한다. GPT-6.1 Sol 병렬 구현 인계 뒤 검증·PR·머지·정리·업데이트를 수행한다.

## 성공 기준
- TESTING과 self-verification에 동일한 필수 검증 기준이 있고 self-verify의 성공 step을 확인하여 test/build/golden/docs/inspect를 중복하지 않는다. 문서-only 최소 검증은 install/daemon/state 쓰기를 요구하지 않는다. 동일 revision/environment/input 단일-run 증거와 별도 필요한 vet/race 조건 및 CI 검증 소유권을 유지한다.

## 비목표
- Go 실행 코드, CI workflow, risk QA 동작 변경은 이 사이클 범위 밖이다.

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
