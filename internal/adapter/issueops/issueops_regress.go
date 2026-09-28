package issueops

import (
	"context"
	"time"

	delegationapp "issueops/internal/application/issueopsdelegation"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RegressIssueOpsForReplan takes the IssueOps feedback loop backward when the
// Brooks devil's advocate (or an equivalent plan review) returns a `stop`
// verdict: it regresses a plan / compatibility-review cycle to grill so the
// scope is re-investigated and the plan redone, rather than blocking in place.
//
// It records the stop reason as a scope decision (audit), clears the rejected
// design's approval so re-entry forces a genuine re-review, and marks the
// downstream plan/compatibility-review ledger entries stale (retained as audit
// per the backward-regression rule). It does not delete the worktree, branch,
// or remote artifacts.
func RegressIssueOpsForReplan(stateRoot, id, reason string) (issueops.IssueOpsRecord, error) {
	return regressIssueOpsForReplan(stateRoot, id, reason, nil)
}

func RegressIssueOpsForReplanWithActor(stateRoot, id, reason string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return regressIssueOpsForReplan(stateRoot, id, reason, &actor)
}

func regressIssueOpsForReplan(stateRoot, id, reason string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var err error
	reason, err = reviewdomain.NormalizeRegressionReason(reason)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	var rec issueops.IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = reviewapp.Regress(reviewport.RegressStore{
			Read:           ReadIssueOps,
			ActiveChildren: (delegationapp.ChildGates{Scan: ScanReadableIssueOps}).ActiveIDs,
			TouchWrite:     touchAndWriteIssueOps,
			Now:            func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
		}, stateRoot, id, reason)
		return e
	})
	return rec, err
}
