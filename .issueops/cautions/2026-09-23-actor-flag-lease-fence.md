---
name: 2026-09-23-actor-flag-lease-fence
description: Caution record for a solved false case or recurring risk.
---

# actor flag를 파싱하고 버리면 lease fence가 조용히 빠진다

- Date: 2026-09-23
- Kind: `caution`
- Source: architecture review 2026-09-23
- Summary: implementation-review, project-docs-review, schema-evidence record가 --host·--session-id·--cwd를 받고도 core에 넘기지 않아, 다른 세션이 활성 lease를 쥔 상태에서도 publication 게이트 증거가 기록됐다.
- Context: 2026-08-10 이후 기본 hook은 IssueOps mutation을 막지 않는다. 그래서 CLI가 넘긴 actor를 core가 검증하는 경로가 유일한 쓰기 경계다. 카탈로그와 스킬은 세 명령에 actor flag를 광고했지만 handler는 addIssueOpsActorFlags(fs)의 반환값을 받지 않았고, core 함수에는 actor 인자가 없었다.
- Resolution: 세 handler가 파싱한 actor를 넘기고, core의 Record*WithActor가 span 안에서 validatePostTransferMutation을 적용한다. cmd/issueops/issueopscli/issueops_actor_flags_test.go는 addIssueOpsActorFlags·addActorFlags의 반환값을 버리는 호출을 AST로 거부한다. 새 owner mutation을 추가할 때는 actor 없는 core 함수만 만들지 말고 WithActor 경로와 fence 테스트를 함께 둔다.
- Evidence:
  - internal/adapter/issueops/issueops_evidence_recorder_fence_test.go: TestEvidenceRecordersRequireTheActiveLeaseHolder
  - cmd/issueops/issueopscli/issueops_actor_flags_test.go: HEAD에서 issueops_subcommands.go:441,471,499 세 곳을 잡는다
