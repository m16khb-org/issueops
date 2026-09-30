package issueopsapp

import (
	core "issueops/internal/adapter/issueops"
	delegation "issueops/internal/application/issueopsdelegation"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

func newPlanningRecorder(actor *model.IssueOpsActor) app.PlanningRecorder {
	readiness := newCycleReadiness()
	return app.PlanningRecorder{ActiveChildren: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).ActiveIDs, Store: core.NewReviewMutationStore(actor), PlanReadiness: readiness.Plan, CompatibilityReadiness: readiness.Compatibility, PhaseRank: domain.IssueOpsPhaseRank, PlanDigest: app.NewPlanDigestResolver(core.ReviewPlanSource{}).Digest}
}
