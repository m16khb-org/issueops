package issueopsreview

import (
	"slices"
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
)

// ImplementationReviewMissing returns the gate key blocking implementation review, or
// "" when it passes. No execution lease means no gate; otherwise the review must be
// present, pass, and match currentFingerprint when one is known.
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

// ProjectDocsReviewMissing returns the gate key for a missing or stale project docs
// review, or "" when it is present and matches currentFingerprint.
func ProjectDocsReviewMissing(review reviewcontract.ReviewGateEvidence, currentFingerprint string) string {
	if !review.Present {
		return "project_docs_review"
	}
	if currentFingerprint != "" && review.ReviewedFingerprint != currentFingerprint {
		return "project_docs_review_stale"
	}
	return ""
}

// SchemaEvidenceMissing returns the gate key for missing or stale schema evidence, or
// "" when satisfied. It applies only when the change touches schema; evidence needs
// measurements and sources, or a waiver with a rationale.
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

// ChangeSetTouchesSchema reports whether any changed path is a schema change.
func ChangeSetTouchesSchema(changed []string) bool {
	return slices.ContainsFunc(changed, issueopsdomain.PathIsSchemaChange)
}
