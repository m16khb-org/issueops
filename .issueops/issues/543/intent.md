# 요청자 의도 계약

- lifecycle: io-ca6dfd9d657f
- issue: https://github.com/m16khb-org/issueops/issues/543
- intent_class: standard

## 원문 요청
1,2,3을 전부 $issueops 로 6.1 sol 병렬인계로 개선을 진행해주고 머지하고 $issueops-cleanup 후 $io-update 까지 해줘

## 해석
개선 1: self-verify가 명시한 기준 커밋부터 HEAD까지의 커밋 변경과 미커밋 변경을 함께 판정하도록 확장한다. 독립 사이클로 검증·PR을 발행하고 coordinator가 머지·정리·업데이트를 수행한다.

## 성공 기준
- clean committed Go diff가 지정 기준 범위에서 vet/race 명령을 선택한다.
- 커밋·staged·unstaged·untracked 변경의 합집합을 판정하며 잘못된 기준은 정상 성공으로 처리하지 않는다.
- CLI/MCP 같은 기준 입력이 같은 risk plan과 기준·HEAD 관측 근거를 남긴다.

## 비목표
- CI vet/race 독립 검사와 기존 무기준 실행 동작은 유지한다.

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
