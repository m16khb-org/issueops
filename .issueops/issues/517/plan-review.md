# 계획 검토 기록

## 1차: 수정 요청

- Reviewer: claude-opus-5 effort high(1라운드, next.review 기본값), claude -p 빈 컨텍스트 세션.
- R1-1: 삽입 문구가 effort 단계를 high→xhigh→max로 하드코딩해 코드가 소유한 docs-only effort(internal/port/orca.go:25-40, IssueOpsReviewEffortDocsOnly=medium)와 충돌하고, 스킬 34-35행의 '표를 복사하지 않는다' 규칙을 어긴다. G3도 xhigh를 필수 토큰으로 못박는다.
- R1-2: golden 재생성 전제가 사실과 다르다. docsIndexContractProjection(response_contract_docs_projection_helper_test.go:23-47)은 docs_count를 정규화해 담을 뿐이라 .md 추가로 golden이 바뀌지 않는다. Deliverable·C2·QA의 golden 항목이 성립하지 않는다.
- R1-3: G10(go test ./...)의 900초 제한이 전체 스위트 시간보다 짧을 수 있다(check.go:156-200). 실측 후 제한을 정해야 한다.
- R1-4: G7이 ADR 색인 링크가 실제 새 파일명을 가리키는지 확인하지 않아 자리표시자가 남아도 통과한다.
- R1-5: 첫 하위 항목에 host 한정어가 없어 codex·omo의 3라운드 모델 선택까지 제한한다(intent 비목표 위반).

## 2차: 수정 요청

- Reviewer: claude-opus-5 effort high(2라운드 delta, next.review 기본값), claude -p 빈 컨텍스트 세션.
- R1-1, R1-2, R1-4, R1-5 해소 확인. R1-3은 G10만 다뤘다.
- R2-1: 계획이 근거로 든 로컬 재현 로그에서 self-verify가 819초 FAIL인데, G13의 시간(상한 900초, policy_evaluate.go:73)과 실패 시 처리를 정하지 않았다. DoD(G1~G13 한 run 통과)와 TESTING의 단일 run 규칙을 지킬 근거가 없다.

## 3차: 통과

- 3라운드 리뷰어: claude-opus-5 effort xhigh(next.review.model과 같은 모델에 effort 한 단계 상향), claude -p 빈 컨텍스트 세션, delta 리뷰. Fable은 쓰지 않았다.
- R2-1 해소: 로컬 819초 self-verify 실패는 Go 테스트 파일 변경으로 elevated tier가 된 race 단계(risk_qa_plan.go:40-43, 10분 제한 risk_qa.go:15)의 시간 초과였고, 이 사이클의 변경은 전부 .md라 같은 경로가 실행되지 않는다(TestPlanRiskQATierFromPaths의 docs only 케이스 PASS). 같은 날 CI run 35962833554의 self-verify 게이트는 116초에 통과했다.
- G13 선실행·600초 트리거·원인별 재실행·부분 통과 조합 금지가 계획에 명시됐다. delta는 3개 hunk이고 게이트 spec부터 끝까지 2차본과 바이트 단위로 같다.
