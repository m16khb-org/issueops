package issueops

import (
	"fmt"

	model "issueops/internal/contract/issueops"
)

// ApplyLinkedBranchCleanupAudit binds an observation to its original cycle and
// prepared target while preserving unrelated changes in the current record.
func ApplyLinkedBranchCleanupAudit(observed, current model.IssueOpsRecord, result model.CleanupLinkedBranchResult, now string) (model.IssueOpsRecord, error) {
	before, after := observed.BranchPrepare, current.BranchPrepare
	if observed.ID != current.ID || observed.Repo != current.Repo ||
		observed.CreatedAt != current.CreatedAt || observed.IssueURL != current.IssueURL || observed.Branch != current.Branch ||
		before == nil || after == nil || before.Provider != after.Provider ||
		before.IssueURL != after.IssueURL || before.Branch != after.Branch ||
		before.BaseSHA != after.BaseSHA || before.BaseBranch != after.BaseBranch || before.CreatedAt != after.CreatedAt {
		return current, fmt.Errorf("linked branch cleanup audit target changed during observation")
	}
	current.LinkedBranchCleanup = &model.IssueOpsLinkedBranchCleanup{
		State: result.State, StateReason: result.StateReason, LinkedBranchID: result.LinkedBranchID,
		LinkedCount: result.LinkedCount, RemoteRefOID: result.RemoteRefOID, Fingerprint: result.Fingerprint,
		Deleted: result.Deleted, AlreadyAbsent: result.AlreadyAbsent, FailedStep: result.FailedStep, ObservedAt: now,
	}
	return current, nil
}
