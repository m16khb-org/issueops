---
name: 2026-09-29-cleanup-finish-executor-owns-observation-and-attempt-bound-f
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Cleanup finish executor owns observation and attempt-bound finalization

- Date: 2026-09-29
- Kind: `adr`
- Source: DDD responsibility refactor T08
- Summary: Application owns cleanup finish ordering; domain owns attempt transitions and persistence binds every receipt and deletion to an exact revision.
- Context: Supersedes only the pending-executor status of 2026-09-29-cleanup-finish-ownership-is-an-optional-record-field-with-gu.md. Its optional schema1 field and older-reader incompatibility decision remain in force.
- Decision: Compose FinishExecutor directly in the root and remove production CleanupFinish. Acquire one stable per-cycle lifetime lock before observations. Derive merge and issue evidence in FinishEvidenceReader from the loaded record; status delegates to the same preview instead of reading remote evidence twice. Arm the exact raw revision before effects. Guard each destructive stage, stamp confirmed provider audit with the same attempt, and drain inherited command descriptors before clearing ownership or atomically deleting record and artifact-stage rows. Drain failure retains the attempt; a later exclusive preview/apply retries without TTL takeover or a recovery command. Shared SQLite spans cover only conditional local transactions.
- Consequences: FinishRecordStore uses CompareAndApplyFunc even for read-only Check because CompareAndApply skips an empty mutation set. A replacement attempt cannot be finalized by the previous owner during the Drain reacquisition gap. Provider audit remains best-effort; local ownership errors block deletion. Remaining workspace, abandon, remote/orphan cleanup policies and transport provenance/provider selection remain separate DDD work; this decision does not mark T08 or the whole refactor complete.
- Evidence:
  - internal/application/issueopscleanup/finish_executor.go
  - internal/application/issueopscleanup/finish_evidence.go
  - internal/application/issueopscleanup/status.go
  - internal/adapter/issueops/cleanup_finish_records.go
  - internal/adapter/issueops/cleanup_finish_concurrency_test.go
  - internal/adapter/issueops/cleanup_finish_inherited_command_test.go
  - cmd/issueops/issueopscli/feedbackcleanup/feedback_cleanup_test.go
- Alternatives / rejected options:
  - Do not keep the production forwarding facade or duplicate status readback.
  - Do not hold the shared database span across process or provider calls.
  - Do not clear ownership based on elapsed time or a successful direct-child exit alone.
  - Do not use an ID-only audit receipt or unconditional final deletion.
