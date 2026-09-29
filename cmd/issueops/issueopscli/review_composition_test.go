package issueopscli

import (
	core "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	port "issueops/internal/port/issueopsreview"
)

func recordImplementationReviewForTest(root, id string, req model.IssueOpsImplementationReviewRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return app.RecordImplementationReview(core.NewEvidenceReviewStore(&actor, testChangeReader().ChangeFingerprint), root, id, req)
}
func recordSchemaEvidenceForTest(root, id string, req model.IssueOpsSchemaEvidenceRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return app.RecordSchemaEvidence(core.NewEvidenceReviewStore(&actor, testChangeReader().ChangeFingerprint), root, id, req)
}
func recordProjectDocsReviewForTest(root, id string, req model.IssueOpsProjectDocsReviewRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	return app.RecordProjectDocsReview(port.ProjectDocsReviewStore{
		EvidenceReviewStore: core.NewEvidenceReviewStore(&actor, testChangeReader().ChangeFingerprint),
		ChangedPaths:        testChangeReader().ChangedPaths, Root: core.ReviewDocumentPaths{}.Root,
		RelativePath: core.ReviewDocumentPaths{}.RelativePath, FileExists: core.ReviewDocumentPaths{}.FileExists,
	}, root, id, req)
}
