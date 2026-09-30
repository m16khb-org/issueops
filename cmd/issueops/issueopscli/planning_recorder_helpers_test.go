package issueopscli

import (
	core "issueops/internal/adapter/issueops"
	cycleapp "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

func planningRecorderForTest(actor *model.IssueOpsActor) app.PlanningRecorder {
	return app.PlanningRecorder{ActiveChildren: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).ActiveIDs, Store: core.NewReviewMutationStore(actor), PlanReadiness: (cycleapp.Readiness{}).Plan, CompatibilityReadiness: (cycleapp.Readiness{Paths: core.ReadinessPathObservations()}).Compatibility, PhaseRank: domain.IssueOpsPhaseRank, PlanDigest: app.NewPlanDigestResolver(core.ReviewPlanSource{}).Digest}
}

func recordDomainReviewForTest(root, id string, req model.IssueOpsDomainReviewRequest) (model.IssueOpsRecord, error) {
	return app.RecordDomainReview(core.NewReviewMutationStore(nil), root, id, req)
}
