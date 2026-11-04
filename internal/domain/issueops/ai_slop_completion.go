package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// AISlopCleanCompletionMissing lists the AI-slop-clean completion fields
// still absent from record: timestamp, head, fingerprint, cleanup evidence,
// and verification evidence. An empty result means the stage can complete.
func AISlopCleanCompletionMissing(record model.Record) []string {
	missing := []string{}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean_at")
	}
	if strings.TrimSpace(record.AISlopCleanHead) == "" {
		missing = append(missing, "ai_slop_clean_head")
	}
	if strings.TrimSpace(record.AISlopCleanFingerprint) == "" {
		missing = append(missing, "ai_slop_clean_fingerprint")
	}
	if !hasUsableEvidence(record.AISlopCleanCategories) {
		missing = append(missing, "cleanup_evidence")
	}
	if !hasUsableEvidence(record.AISlopCleanVerification) {
		missing = append(missing, "verification_evidence")
	}
	return missing
}
