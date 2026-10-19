package issueops

import (
	"strings"

	cycleapp "issueops/internal/application/issueopscycle"
	delegation "issueops/internal/application/issueopsdelegation"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	domain "issueops/internal/domain/issueops"
)

func planningRecorderForTest(actor *model.IssueOpsActor) app.PlanningRecorder {
	return app.PlanningRecorder{ActiveChildren: (delegation.ChildGates{Scan: ScanReadableIssueOps}).ActiveIDs, Store: NewReviewMutationStore(actor), PlanReadiness: (cycleapp.Readiness{}).Plan, CompatibilityReadiness: (cycleapp.Readiness{Paths: ReadinessPathObservations()}).Compatibility, PhaseRank: domain.IssueOpsPhaseRank, PlanDigest: app.NewPlanDigestResolver(ReviewPlanSource{}).Digest, ArchitectureDocument: ReviewArchitectureSource{}.Document}
}
func RecordIssueOpsIntent(root, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Intent(root, id, req)
}

// RecordIssueOpsArchitectureReview는 테스트 편의 래퍼다. 문서가 아직 스테이징되지 않았으면
// 필수 절을 채운 문서를 스테이징하고, reviewer_context가 비어 있으면 subagent로 둔다.
func RecordIssueOpsArchitectureReview(root, id string, req reviewcontract.ArchitectureReviewRequest) (model.IssueOpsRecord, error) {
	if staged, err := readStagedArtifacts(root, id); err == nil && staged["architecture"] == "" {
		if record, readErr := ReadIssueOps(root, id); readErr == nil && record.Execution == nil {
			if _, stageErr := stageIssueOpsArtifactForTest(root, id, "architecture", []byte(architectureDocumentForTest())); stageErr != nil {
				return model.IssueOpsRecord{}, stageErr
			}
		}
	}
	if req.ReviewerContext == "" {
		req.ReviewerContext = "subagent"
	}
	return planningRecorderForTest(nil).Architecture(root, id, req)
}

func architectureDocumentForTest() string {
	return strings.Join(domain.RequiredArchitectureSections, "\n내용\n") + "\n내용\n"
}
func RecordIssueOpsCompatibilityReview(root, id string, req reviewcontract.CompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).Compatibility(root, id, req)
}
func RecordIssueOpsDevilsAdvocateReview(root, id string, req reviewcontract.DevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	return planningRecorderForTest(nil).DevilsAdvocate(root, id, req)
}
