---
name: 2026-09-29-cleanup-ownership-binds-the-operation-and-exact-record-revis
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Cleanup ownership binds the operation and exact record revision

- Date: 2026-09-29
- Kind: `adr`
- Source: DDD responsibility refactor T08
- Summary: Finish, remote-branch and abandon share one operation-bound cleanup attempt and inherited-process lifetime, while their final effects remain distinct.
- Context: Remote branch cleanup accepted a changed artifact after observation, permitted overlapping executors and ordinary writers during deletion, and could acknowledge audit against a changed cycle. T08-remote-ownership-red.txt reproduced all three cases. The finish-only attempt was introduced on this unpublished refactor branch.
- Decision: Replace the unpublished cleanup_finish_attempt field with the single canonical optional cleanup_attempt containing required operation (finish, remote-branch or abandon), token and started_at. Preserve schema1 records without attempts; reject the retired draft field and unknown operations instead of maintaining aliases. Domain owns operation access, attempt arming and ownership predicates. Application owns acquire, observe, exact-raw arm, check, external effects, bound audit receipt, drain and finalization. CleanupRecordStore applies raw/token/operation CAS and restricts local record Delete/Fail to finish and Release to remote-branch. All three executors use the same lifetime lock; keep its physical cleanup-finish-locks namespace stable across local binary updates so inherited descriptors continue to exclude replacement executors.
- Consequences: This supersedes the finish-only field and store naming in the 2026-09-29 finish-ownership and finish-executor decisions; their raw CAS, all-writer fence and drainage requirements still apply. Preview is read-only. An absent remote ref without an attempt remains a no-write success before confirmation/fingerprint; explicit apply may recover and release only a crashed remote-branch attempt after successful drainage, without claiming a deletion or audit. A foreign operation must recover through its own command. Provider audit failure remains best-effort, but stale receipt or drainage failure fails finalization and preserves ownership. Abandon uses a sealed-inventory ArmAbandon, its own failure finalizer and atomic deletion of owned intent rows, staged artifact and record. It acquires the shared lifetime before provider/artifact observations and refuses raw drift before effects. Local cancellation preserves the sealed applying receipt for fresh resource observation; an applying receipt alone cannot authorize concurrent execution. Abandon preview/pending/Orca policy migration, orphan migration and full DDD delivery remain separate unfinished work.
- Evidence:
  - internal/contract/issueops/cleanup_attempt.go
  - internal/domain/issueops/cleanup_attempt.go
  - internal/application/issueopscleanup/remote_branch.go
  - internal/adapter/issueops/cleanup_records.go
  - internal/adapter/issueops/cleanup_lifetime.go
  - TestCleanupRemoteBranchRejectsArtifactDriftBeforeDelete
  - TestCleanupRemoteBranchExcludesOtherExecutorsAndWriters
  - TestCleanupRemoteBranchPreservesRecordChangedDuringAudit
  - TestCleanupRemoteBranchRetainsAttemptUntilInheritedGitChildExits
  - TestCleanupRecordsBindOperationAndReplacementAcrossFinalizers
  - TestCleanupOperationCodecBindsSupportedOperations
  - TestCleanupAttemptRejectsRetiredDraftField
  - Independent design-review verdict proceed; remote-branch-ownership-review.md
  - internal/application/issueopscleanup/abandon_executor.go
  - internal/adapter/issueops/cleanup_abandon_records.go
  - TestCleanupAbandonCannotRecoverAnExecutingAttempt
  - TestAbandonCLIObservesOnlyAfterOwnershipAndBindsLoadedArtifact
  - TestAbandonCancellationAfterLocalDeleteRecoversFromFreshInventory
  - TestAbandonInheritedChildrenRetainExecutionOwnership
- Alternatives / rejected options:
  - A second remote-specific attempt and duplicated writer guards would create two authorities for the same cycle.
  - Re-reading once before Git without excluding ordinary writers would leave the effect race open.
  - Holding the SQLite span across Git/provider calls would serialize unrelated cycles and violate the existing network boundary.
  - A journal, TTL takeover, new recovery CLI or compatibility alias is unnecessary for these existing operations.
