package intentdesign

import (
	"time"

	intentapp "issueops/internal/application/issueopsintent"
	model "issueops/internal/contract/issueops"
	intentport "issueops/internal/port/issueopsintent"
)

// RecordPlanPrep stores the pre-plan evidence gate: prior-decision lookup,
// related-issue scoring, web research, and the codebase survey. Each item must
// carry either evidence or a waive reason (mutually exclusive). The plan
// readiness gate then checks these for non-trivial intent classes.
func RecordPlanPrep(store Store, stateRoot, id string, req model.IssueOpsPlanPrepRequest) (model.IssueOpsRecord, error) {
	return intentapp.RecordPlanPrep(intentport.Store{
		Read:       store.Read,
		TouchWrite: store.TouchWrite,
		Now:        func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}, stateRoot, id, req)
}
