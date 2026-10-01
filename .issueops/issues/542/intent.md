# 요청자 의도 계약

- lifecycle: io-ebcba1cf345f
- issue: https://github.com/m16khb-org/issueops/issues/542
- intent_class: standard

## 원문 요청
1,2,3을 전부 $issueops 로 6.1 sol 병렬인계로 개선을 진행해주고 머지하고 $issueops-cleanup 후 $io-update 까지 해줘

## 해석
개선점 2: 감사표 수집에서 헤더와 행을 검증하고 명시적 0건과 수집 실패를 구분한다. 이 독립 사이클은 감사표 수집 변경만 담당한다.

## 성공 기준
- 현재 실제 PROJECT_AUDIT.md는 정상 수집 0건; 헤더 순서가 다른 P0/P1/P2 열린 항목은 정확히 수집; 누락·빈 파일·알 수 없는 형식·잘못된 행은 warning으로 collection=error health=unknown gate=block; 이력 표는 제외; 공개 CLI/MCP JSON 필드 유지.

## 비목표
- 검증 위험도 계산 및 self-verification 문서 정리는 다른 독립 사이클 소유

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
