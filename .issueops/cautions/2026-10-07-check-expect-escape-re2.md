---
name: 2026-10-07-check-expect-escape-re2
description: Caution record for a solved false case or recurring risk.
---

# 게이트 원장 CHECK·EXPECT 작성 함정: escape, RE2, 글자 대리 검사

- Date: 2026-10-07
- Kind: `caution`
- Source: #548
- Summary: #548 게이트 원장에서 기대값은 맞는데 원장 작성 방식 때문에 미충족으로 남은 경우가 세 번 나왔다.
- Context: 1) CHECK에 넣은 python3 -c 문자열의 백슬래시 n을 원장 파서가 실제 줄바꿈으로 풀어 SyntaxError가 났다. 2) EXPECT의 /정규식/은 Go RE2라 lookahead (?!...)를 지원하지 않아 COLLECTION_STATUS=ok 출력에도 매치되지 않았다. 3) internal/holdoutdeleak 테스트가 .gitignore에 evidence라는 글자 그대로의 줄이 있는지만 보는 대리 검사여서, 정답 트리를 그대로 무시하는 .issueops/evidence/ 규칙으로 바꾸자 실패했다. CHECK의 '|'는 세그먼트 구분자라 쓸 수 없고, gates check의 --timeout-seconds 상한은 900이다.
- Resolution: CHECK 안의 줄바꿈은 chr(10)으로 만들고 '|'는 쓰지 않는다. EXPECT 정규식은 RE2 문법만 쓰고, 값 집합이 작으면 정확한 줄(예: COLLECTION_STATUS=ok)로 쓴다. 파일 내용의 글자 대신 효과를 검사한다(ignore는 git check-ignore --no-index, GIT_CONFIG_GLOBAL=os.DevNull로 개인 excludesFile 배제). 원장을 고친 뒤에는 단일 실행 증거를 위해 전체 원장을 다시 돌린다.
