package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

// SchemaEvidenceMissingForPaths 함수는 changed 경로가 스키마를 건드릴 때 스키마 증거가 없거나 현재 지문과 맞지 않으면 누락 키를, 아니면 빈 문자열을 돌려준다.
func SchemaEvidenceMissingForPaths(record model.Record, changed []string, currentFingerprint string) string {
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

// ObservedSchemaEvidenceMissing 함수는 실행 기록에 대해 스키마 증거 누락 키를 돌려준다. strict이면 observePaths로 변경 경로를 새로 관찰하고, 아니면 observedPaths를 쓴다. 실행이 없으면 빈 문자열이다.
func ObservedSchemaEvidenceMissing(record model.Record, strict bool, observedPaths []string, currentFingerprint string, observePaths func(model.Record) []string) string {
	if record.Execution == nil {
		return ""
	}
	if strict {
		observedPaths = observePaths(record)
	}
	return SchemaEvidenceMissingForPaths(record, observedPaths, currentFingerprint)
}
