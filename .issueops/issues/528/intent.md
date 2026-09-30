# 요청자 의도 계약

- lifecycle: io-0b57c36d0e6f
- issue: https://github.com/m16khb-org/issueops/issues/528
- intent_class: standard

## 원문 요청
진행해봐

## 해석
제안한 후속 작업 중 self-verify와 CI의 저장소 소유 Python 검사 불일치를 해소한다.

## 성공 기준
- Python 검사 실패가 로컬 self-verify 실패로 이어지고, 통과 시 명시적 step과 점수·coverage에 반영된다. 인터프리터 누락·비호환 진단을 확인한다.

## 비목표
- 공개 문서 경로 정규화 구현과 hostprobe 간헐 실패 수정, 설치 변경

## 제약
- 검사 약화와 공개 문서 개인정보 스캔 축소 금지

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
