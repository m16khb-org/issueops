package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// Regress runs inside the caller's existing cycle lock and persists the entire
// backward transition only after both cycle and child preconditions pass.
func Regress(store reviewport.RegressStore, stateRoot, id, reason string) (model.IssueOpsRecord, error) {
	reason, err := reviewdomain.NormalizeRegressionReason(reason)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if err := reviewdomain.CheckRegression(reviewcontract.RegressionPreconditions{
		CycleID: id, Phase: string(record.Phase), Review: record.DevilsAdvocateReview, RegressCount: len(record.RegressEvents),
	}); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	activeChildren, err := store.ActiveChildren(stateRoot, record)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if err := reviewdomain.CheckRegressionChildren(activeChildren); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	change := reviewdomain.BuildRegressionChange(string(record.Phase), reason, store.Now())
	record.RegressEvents = append(record.RegressEvents, model.IssueOpsRegressEvent{
		Reason: change.EventReason, FromPhase: model.IssueOpsPhase(change.FromPhase), At: change.At,
	})
	record.Decisions = append(record.Decisions, model.IssueOpsDecision{
		Title: change.DecisionTitle, Body: change.DecisionBody, Kind: change.DecisionKind,
		Rationale: change.DecisionRationale, CreatedAt: change.At,
	})
	if change.ClearDesignApproval && record.DesignReview != nil {
		record.DesignReview.Approved = false
	}
	stalePhases := make([]model.IssueOpsPhase, 0, len(change.StalePhases))
	for _, phase := range change.StalePhases {
		stalePhases = append(stalePhases, model.IssueOpsPhase(phase))
	}
	record.PhaseLedger = issueopsdomain.MarkLedgerStale(record.PhaseLedger, change.StaleNote, stalePhases...)
	if change.ClearReview {
		record.DevilsAdvocateReview = nil
	}
	record.Phase = model.IssueOpsPhase(change.ToPhase)
	record.UpdatedAt = change.At
	return store.TouchWrite(stateRoot, record)
}
