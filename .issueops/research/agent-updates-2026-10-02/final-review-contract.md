# Fable 5.1 최종 독립 검증

사용자 추가 요청: “다 조사한 후에 fable 5.1로 한번 검증해보는게 좋겠어”

전체 수집, 후속 질문 정리, 한국어 개선안 초안이 끝난 뒤 **Fable 5.1**로
독립 검증을 한 번 수행한다. 기존 Luna·Sol·Astra 배분은 수집·분석 단계에
유지한다.

## 검토 입력

- 최종 한국어 보고서와 근거 색인.
- 버전 고정 여부, 원문 조회일, 설치본과 공개 릴리스의 차이.
- 실제 issueops 코드 참조와 기존 기능 목록.
- 우선순위, 비용·리스크, 제안별 검증 실험.
- 반증 기록과 unresolved 항목.

## 검토 질문

1. 최신 릴리스와 preview/draft, API와 CLI, 다른 배포판을 혼동했는가?
2. 이미 구현된 기능을 누락으로 잘못 판단했는가?
3. 측정하지 않은 성능 효과나 비용 절감을 사실처럼 표현했는가?
4. MCP 2025-11-25와 2026-07-28의 세션·취소·알림 계약을 구별했는가?
5. Streamable HTTP가 issueops의 native actor·generation 검증을 보존한다는
   근거가 있는가?
6. 제안이 Go core·얇은 adapter·context-only hook 원칙을 불필요하게 바꾸는가?
7. 우선순위와 실험 기준이 실제 근거에서 도출되는가?

검토자는 보고서나 코드를 직접 바꾸지 않고 심각도·근거·수정 제안과 판정을
반환한다. 메인이 지적을 검증하고 필요한 수정을 반영한다.
실제 Fable 5.1 모델 식별자와 실행 영수증을 확인하며, 기본 architect
카테고리나 다른 Fable 버전으로 조용히 대체하지 않는다.

## 모델 식별자 확인

2026-10-02 `omo --list-models fable`이 exit 0으로 반환한 목록에
`anthropic-subscription / claude-fable-5-1`이 있다.
최종 검토에는 `anthropic-subscription/claude-fable-5-1`을 명시한다.
일부 다른 provider는 점 표기 `claude-fable-5.1`을 쓰므로 이름을 섞지 않는다.
이 목록은 등록명 확인이며, 실제 검토 실행 성공은 작업 영수증으로 별도 확인한다.

## 실행 결과

`st_01a0fb21`이 지정한 Fable 5.1로 완료됐다.
ready with corrections 판정과 6개 지적을 받았으며, 메인이 원문·코드로 확인해
반영했다. [검토와 반영 결과](fable-5.1-review.md)를 참조한다.
두 번째 Fable 검토는 실행하지 않았다.
