# 요청자 의도 계약

- lifecycle: io-c63caa7ea2ee
- issue: https://github.com/m16khb-org/issueops/issues/534
- intent_class: standard

## 원문 요청
5개작업 $issueops 로 병렬 인계 작업해줘 완료되면 다시 main에 머지하고 $issueops-cleanup 하고 $io-update 까지 진행. 인계작업 세션은 6.1 sol high로 진행되는것 맞지?

## 해석
기존 #534에서 확인한 golden 미실행 성공 표시와 doctor binary drift 성공 오판을 수정하고 검증한다. GPT-6.1 Sol high native owner는 draft PR 및 execution complete까지 수행하고 조정 세션이 main 병합, cleanup, io-update를 마친다.

## 성공 기준
- 실제 CLI/MCP/response golden 네 테스트 실행을 증명하고 zero-match는 실패하며 full-suite reuse를 유지한다. binary_drift stale/fresh/missing/malformed 및 필드 누락을 의미대로 판정하고 무관한 doctor 경고는 무시한다. argv만 확인하는 mock 외 실제 실패 fixture, 독립 리뷰, 최신 PR CI 및 execution complete 증거가 있다.

## 비목표
- 이 owner는 main merge, cleanup, global install/update를 하지 않는다. 조정 세션이 맡는다.

## 제약
- 기존 doctor exit code, generic runner, CLI/MCP/record schema를 유지한다. #535 CI 및 #536 loop metadata를 침범하지 않는다.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
