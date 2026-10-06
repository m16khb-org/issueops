---
name: 2026-10-06-removal-commits-leave-stale-help-text-orphan-fixtures-and-do
description: Caution record for a solved false case or recurring risk.
---

# Removal commits leave stale help text, orphan fixtures, and doc claims behind

- Date: 2026-10-06
- Kind: `caution`
- Source: cli
- Summary: 기능·경로를 지우는 커밋은 코드만 지우고 도움말 문자열, testdata golden, 다른 패키지의 docs index 제외 규칙, 생성 문서를 그대로 남기기 쉽다.
- Context: 5d90885a가 --field alias를 지웠지만 remotecmd 도움말 4곳은 'canonical or documented alias'를 계속 안내했다. draft-wiki 명령 제거 뒤에도 project usage 줄, internal/domain/docs/selection.go의 .issueops/draft-wiki 제외 규칙, .issueops/draft-wiki/ 디렉터리가 남았다. 코드 참조가 끊긴 golden 9개(legacy_create_pr_*, claimvertical/*, remote_publication_v1/*)는 테스트가 읽지 않아 CI가 잡지 못했다. deadcode는 함수만 보므로 test-only exported 타입·alias·인터페이스 집계는 따로 찾아야 했다.
- Resolution: 제거 커밋 전에 지운 이름(명령, 플래그, 경로, 디렉터리, MCP tool)으로 git grep을 Go 문자열·usage golden·testdata·skills·.issueops 현행 문서·configs까지 돌린다. testdata는 파일명이 코드에 한 번도 안 나오면 고아로 본다. 함수 단위는 golang.org/x/tools/cmd/deadcode ./cmd/...로, 타입·alias는 production 참조 0 스캔으로 확인한다. 날짜가 붙은 기록 문서는 고치지 않고, 현행처럼 읽히는 문서만 고친다.
