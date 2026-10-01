# 요청자 의도 계약

- lifecycle: io-1fa62d908157
- issue: https://github.com/m16khb-org/issueops/issues/535
- intent_class: standard

## 원문 요청
5개작업 $issueops 로 병렬 인계 작업해줘 완료되면 다시 main에 머지하고 $issueops-cleanup 하고 $io-update 까지 진행. 인계작업 세션은 6.1 sol high.

## 해석
CI와 self-verify 중복 검사 비용을 커버리지 손실 없이 줄이고 검증된 PR을 생성한다. Root coordinator가 병합·정리·업데이트한다.

## 성공 기준
- - [ ] Python·일반 Go·race·golden·native integration의 실행 책임을 표로 정하고 동일 검사 반복이 필요한 환경 차이를 명시합니다.
- [ ] 동일 실행·소스·환경의 검사를 한 곳에서 수행하도록 구성하거나 검증된 증거를 재사용합니다. SHA·dirty 상태·실행 범위가 다른 증거는 거부합니다.
- [ ] race, 임시 HOME의 native integration, 실패 전파를 생략하거나 gate를 약화하지 않습니다.
- [ ] Python 실패와 Go/race 실패가 CI를 실패시키는지 검증하고, 독립 실행하는 로컬 self-verify는 필요한 검사를 모두 수행합니다.
- [ ] 같은 조건의 CI 전후 실행 횟수와 단계별 시간을 기록해 감소를 입증합니다. 근거 없이 고정 절감률을 약속하지 않습니다.

## 비목표
- (없음)

## 제약
- 구현자는 Draft PR과 최신 push/PR CI 확인 및 execution complete까지. main merge, cleanup, 전역 설치는 root coordinator 소유.
- #534 golden/binary drift 수정과 #536 loop_contract 수정에 침범하지 않는다.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
