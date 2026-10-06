package issueops

import (
	cycleapp "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

func planningRecorderForTest(actor *model.IssueOpsActor) app.PlanningRecorder {
	return app.PlanningRecorder{ActiveChildren: (delegation.ChildGates{Scan: ScanReadableIssueOps}).ActiveIDs, Store: NewReviewMutationStore(actor), PlanReadiness: (cycleapp.Readiness{}).Plan, CompatibilityReadiness: (cycleapp.Readiness{Paths: ReadinessPathObservations()}).Compatibility, PhaseRank: domain.IssueOpsPhaseRank, PlanDigest: app.NewPlanDigestResolver(ReviewPlanSource{}).Digest}
}
func RecordIssueOpsIntent(root, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Intent(root, id, req)
}
func RecordIssueOpsDesignReview(root, id string, req model.IssueOpsDesignReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Design(root, id, req)
}
func RecordIssueOpsCompatibilityReview(root, id string, req model.IssueOpsCompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Compatibility(root, id, req)
}
func RecordIssueOpsDevilsAdvocateReview(root, id string, req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).DevilsAdvocate(root, id, req)
}
