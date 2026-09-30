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
func RecordIssueOpsIntentWithActor(root, id string, req model.IssueOpsIntentRecordRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(&actor).Intent(root, id, req)
}
func RecordIssueOpsPlanPrep(root, id string, req model.IssueOpsPlanPrepRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).PlanPrep(root, id, req)
}
func RecordIssueOpsPlanPrepWithActor(root, id string, req model.IssueOpsPlanPrepRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(&actor).PlanPrep(root, id, req)
}
func RecordIssueOpsDesignReview(root, id string, req model.IssueOpsDesignReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Design(root, id, req)
}
func RecordIssueOpsDesignReviewWithActor(root, id string, req model.IssueOpsDesignReviewRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(&actor).Design(root, id, req)
}
func RecordIssueOpsCompatibilityReview(root, id string, req model.IssueOpsCompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Compatibility(root, id, req)
}
func RecordIssueOpsCompatibilityReviewWithActor(root, id string, req model.IssueOpsCompatibilityReviewRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(&actor).Compatibility(root, id, req)
}
func RecordIssueOpsDevilsAdvocateReview(root, id string, req model.IssueOpsDevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).DevilsAdvocate(root, id, req)
}
func RecordIssueOpsDevilsAdvocateReviewWithActor(root, id string, req model.IssueOpsDevilsAdvocateReviewRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(&actor).DevilsAdvocate(root, id, req)
}
