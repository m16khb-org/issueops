---
name: 2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Delete re-export shims and stop naming live code legacy

- Date: 2026-10-06
- Kind: `adr`
- Source: cli
- Summary: 타입·상수·변수·함수를 다른 패키지 이름으로 다시 내보내던 re-export shim과 테스트 helper의 단순 위임 wrapper를 지우고, 레이어 규칙 때문에 꼭 필요한 re-export만 근거 주석과 함께 남긴다. 지금 동작에 쓰이는 코드에서는 legacy라는 이름을 쓰지 않는다.
- Context: legacy 코드 제거 뒤에도 이름에 legacy가 없는 호환 shim이 남아 있었다. production type alias 114개(type 블록 포함), const 84개, var 10개, 단순 위임 func 76개와 테스트 전용 alias·wrapper 수백 개는 DDD 리팩터링 때 옛 호출부를 살리려고 남긴 것이었다. 반대로 폐기된 이름의 재유입을 막는 가드(forbiddenLegacyNeedles)와 API doc 리뷰의 '기존 문서 부채' 문구는 지금 동작인데 legacy라고 불려 다음 감사에서 잔재로 오인될 수 있었다. compatibility review 단계는 변경의 호환성을 검토한다는 뜻이라 이름이 정확하고, 기록 schema·CLI·skill을 깨는 비용이 커서 사용자 결정으로 유지한다.
- Decision: 1) re-export는 internal/architecture/dependency_test.go의 레이어 규칙상 정식 심볼을 직접 import할 수 없을 때만 둔다(domain은 같은 이름의 vertical contract만 import, completion application은 completion contract만 import 등). 유지한 alias 34개와 const/var 48개에는 그 이유를 주석으로 적는다. 나머지는 사용처를 정식 심볼로 바꾸고 지운다(type 80, const 38, var 8, func 76, 테스트 alias·wrapper 약 420). wrapper만 남은 파일 약 40개와 adapter/failurecause 패키지는 삭제한다. 2) 가드는 retired name으로 부른다(retiredNameNeedles, validation_retired_names.go, 'retired name hits'). API doc 리뷰 문구와 MCP api_doc_review 설명은 'pre-existing debt'로 바꾸고 MCP catalog SHA를 0cc87e9a…로 갱신한다. 3) 지운 facade·wrapper 이름이 남은 테스트 이름을 바꾼다. 4) compatibility review 단계, 기록 필드, CLI 플래그, issueops_cli_mcp_compatibility 계약 이름은 유지한다.
- Consequences: 2026-08-08 port-contract-vocabulary의 'port에는 인터페이스와 그 별칭만 남는다'는 서술 중 별칭 부분은 더 이상 사실이 아니다(port의 contract 재수출 alias는 제거됨). 새 re-export를 추가하려면 레이어 규칙상 필요하다는 근거 주석이 있어야 한다. MCP catalog SHA가 바뀌어 io update로 호스트 설정을 다시 써야 한다. 테스트 seam용 var(GitCmd/GitCmdRaw/GitOut)는 값 주입 지점이라 re-export가 아니므로 남는다.
