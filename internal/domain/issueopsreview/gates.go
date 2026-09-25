package issueopsreview

import (
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

func ImplementationReviewMissing(executionPresent bool, review reviewcontract.ReviewGateEvidence, currentFingerprint string) string {
	if !executionPresent {
		return ""
	}
	if !review.Present {
		return "implementation_review"
	}
	if review.Verdict != "pass" {
		return "implementation_review_verdict_" + review.Verdict
	}
	if currentFingerprint != "" && review.ReviewedFingerprint != currentFingerprint {
		return "implementation_review_stale"
	}
	return ""
}

func ProjectDocsReviewMissing(review reviewcontract.ReviewGateEvidence, currentFingerprint string) string {
	if !review.Present {
		return "project_docs_review"
	}
	if currentFingerprint != "" && review.ReviewedFingerprint != currentFingerprint {
		return "project_docs_review_stale"
	}
	return ""
}

func SchemaEvidenceMissing(touchesSchema bool, evidence reviewcontract.SchemaGateEvidence, currentFingerprint string) string {
	if !touchesSchema {
		return ""
	}
	if !evidence.Present {
		return "schema_evidence"
	}
	if evidence.Waived {
		if strings.TrimSpace(evidence.WaiverRationale) == "" {
			return "schema_evidence"
		}
	} else if evidence.MeasurementCount == 0 || evidence.SourceCount == 0 {
		return "schema_evidence"
	}
	if currentFingerprint != "" && evidence.ReviewedFingerprint != currentFingerprint {
		return "schema_evidence_stale"
	}
	return ""
}

func ChangeSetTouchesSchema(changed []string) bool {
	for _, path := range changed {
		if issueopsdomain.PathIsSchemaChange(path) {
			return true
		}
	}
	return false
}
