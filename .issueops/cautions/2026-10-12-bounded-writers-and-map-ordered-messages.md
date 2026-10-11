---
name: 2026-10-12-bounded-writers-and-map-ordered-messages
description: Caution record for a solved false case or recurring risk.
---

# Bounded writers and map-ordered messages

- Date: 2026-10-12
- Kind: `caution`
- Source: quality autoresearch session (complexity, modernize, coverage stages)
- Summary: `bytes.Buffer`를 임베딩한 출력 상한 writer는 `io.Copy`가 승격된 `ReadFrom`을 호출해 상한을 우회한다. map 리터럴을 순회하는 필수 필드 검증은 여러 필드가 비었을 때 오류 메시지의 필드가 실행마다 바뀐다.
- Context: 2026-09-20 hostprobe에서 고친 `ReadFrom` 우회가 `internal/adapter/cmux` `ExecRunner`에 그대로 남아 1 MiB 상한이 동작하지 않았다(1,100,000바이트 출력이 오류 없이 전부 반환됨). 같은 세션에서 `ValidateIssueCreateIntent`, lease workspace/Orca 검증, handoff delivery 검증 등 12곳이 `for k, v := range map[string]string{...}`로 필수 필드를 검사해 보고 필드가 비결정적이었고, `glab api graphql` argv도 map 순회 순서를 따랐다.
- Resolution: 프로세스 출력 writer는 `struct{ io.Writer }{&buf}`로 감싸 `ReadFrom`을 숨긴다(hostprobe, cmux). 필수 필드 검증은 소스 순서를 유지하는 `[]struct{ name, value string }` 슬라이스로 순회한다. 외부 명령 argv에 map을 펼칠 때는 `slices.Sorted(maps.Keys(m))`로 키를 정렬한다. 상한 writer에는 실제 subprocess로 상한 초과를 재현하는 회귀 테스트를 둔다.
- Evidence:
  - internal/adapter/cmux/client_errors_test.go `TestExecRunnerCapsOutputAtOneMiBAndFails` (수정 전 err=nil로 실패)
  - internal/adapter/provider/gitlab/provider_child.go `runGlabGraphQLContext`
  - internal/contract/issueops/issue_create_test.go 다중 공백 필드 케이스('marker is required')
- Alternatives / rejected options:
  - writer를 `bytes.Buffer` 임베딩 대신 별도 필드로 바꾸기만 하는 방법: 이후 리팩터링에서 다시 임베딩될 위험이 같아 래핑과 회귀 테스트를 함께 둔다
  - 오류 메시지에 모든 누락 필드를 정렬해 나열: 공개 오류 문구가 바뀌므로 채택하지 않았다
