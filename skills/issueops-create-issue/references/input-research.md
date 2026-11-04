# 입력 조사 세부 규칙

`SKILL.md`의 `## 입력 세 가지`가 가리키는 세부 규칙이다. 본문은 세 입력의 조사 출처를 plan-prep evidence
문자열로 남긴다는 계약과 evidence 플래그 이름을 소유하고, 이 문서는 각 입력을 조사하는 방법을 소유한다.

## 1. 사용자가 준 정보

원문을 그대로 intent contract의 `--raw-request`에 넣는다.
요약해서 넣으면 나중에 해석이 맞았는지 대조할 원본이 사라진다.
사용자가 HWP·PDF·DOCX 기획서나 화면 캡처를 줬으면
요구사항·모순·누락을 원문·표·화면·작은 글씨까지 확인해 먼저 추출한다. 설치된
`requirements-analysis`를 활용한다. 원문 요청은 그대로 `--raw-request`에 넣고, 추출한 제약은
`intent record --constraint`에, 모순과 누락은 `--ambiguity`에 넣는다. 문서를 요약해
`--raw-request`를 대체하지 않는다.

## 2. 코드베이스 조사

`.codegraph/`가 있으면 `codegraph explore "<질문>"`으로 관련
심볼과 호출 경로를 찾고, 없으면 `rg`로 찾는다. 만진 심볼·파일·호출 경로를 evidence
문자열로 만든다(`--codebase-survey-evidence`). 운영 문서는 `project_docs_route`에 요청
요약을 넣어 고른다. 이미 내려진 결정(`.issueops/ADR.md`와 `adr/` 기록)과 반복 실수
(`.issueops/CAUTIONS.md`와 `cautions/` 기록) 중 이 요청에 걸리는 항목은 문서 경로와 항목
제목을 `--decisions-evidence`에 적는다. 걸리는 항목이 없으면 대조한 문서 목록을 적는다.
`project_docs_route`는 요청과 단어가 두 개 이상 겹치는 기록만 고르므로, 요청이 새로 들이는 핵심 단어(새
플래그·명령 이름, 동작을 가리키는 영어 단어)는 따로 `.issueops/adr/`와 `.issueops/cautions/`에서 Grep한다.
걸린 기록은 열어서 그 문장이 이 요청의 범위나 방식을 정하는지 판정한다. 정하면 위 evidence와 이슈 본문의
범위·비목표에 옮긴다. 걸렸는데 열어 보지 않고 "해당 없음"으로 적지 않는다.
모든 변경에 걸리는 원칙(`.issueops/CONSTITUTION.md`)도 연다. 요청한 동작이 원칙에 걸리면(예: 반복·재시도·
감시 루프의 상한과 종료 경로, secret 처리, 권한 경계) 그 원칙을 성공 기준이나 제약으로 이슈 본문에
옮기거나, 지키는 방법이 갈리면 blocking 질문으로 묻는다.

## 3. 배경지식과 웹 조사

외부 API의 의미나 계약이 걸릴 때만
출처를 직접 조사하고(설치된 `web-research` 활용) 그 결과를
`--web-research-evidence`에 넣는다. 조사하지 않았으면 waive하지 말고 왜 필요 없는지를
evidence로 쓴다. 관련 이슈는 `--related-score-ref`로 남긴다.
