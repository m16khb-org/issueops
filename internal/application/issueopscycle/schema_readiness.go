package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

func SchemaEvidenceMissingForPaths(record model.IssueOpsRecord, changed []string, currentFingerprint string) string {
	evidence := reviewcontract.SchemaGateEvidence{}
	if recorded := record.SchemaEvidence; recorded != nil {
		evidence = reviewcontract.SchemaGateEvidence{
			Present: true, Waived: recorded.Waived, WaiverRationale: recorded.WaiverRationale,
			MeasurementCount: len(recorded.Measurements), SourceCount: len(recorded.Sources),
			ReviewedFingerprint: recorded.ReviewedFingerprint,
		}
	}
	return reviewdomain.SchemaEvidenceMissing(reviewdomain.ChangeSetTouchesSchema(changed), evidence, currentFingerprint)
}

func ObservedSchemaEvidenceMissing(record model.IssueOpsRecord, strict bool, observedPaths []string, currentFingerprint string, observePaths func(model.IssueOpsRecord) []string) string {
	if record.Execution == nil {
		return ""
	}
	if strict {
		observedPaths = observePaths(record)
	}
	return SchemaEvidenceMissingForPaths(record, observedPaths, currentFingerprint)
}
