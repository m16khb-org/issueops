package issueopscli

import (
	core "issueops/internal/adapter/issueops"
	cycleapp "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

func planningRecorderForTest(actor *model.Actor) app.PlanningRecorder {
	return app.PlanningRecorder{ActiveChildren: (delegation.ChildGates{Scan: core.ScanReadableIssueOps}).ActiveIDs, Store: core.NewReviewMutationStore(actor), PlanReadiness: (cycleapp.Readiness{}).Plan, CompatibilityReadiness: (cycleapp.Readiness{Paths: core.ReadinessPathObservations()}).Compatibility, PhaseRank: domain.PhaseRank, PlanDigest: app.NewPlanDigestResolver(core.ReviewPlanSource{}).Digest, ArchitectureDocument: core.ReviewArchitectureSource{}.Document}
}

func recordDomainReviewForTest(root, id string, req model.DomainReviewRequest) (model.Record, error) {
	return app.RecordDomainReview(core.NewReviewMutationStore(nil), root, id, req)
}
