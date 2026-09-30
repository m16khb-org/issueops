package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func AISlopCleanFingerprintReadinessMissing(record model.IssueOpsRecord, currentFingerprint string) []string {
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		return nil
	}
	storedFingerprint := strings.TrimSpace(record.AISlopCleanFingerprint)
	if storedFingerprint == "" && currentFingerprint != "" {
		return []string{"ai_slop_clean_fingerprint"}
	}
	if storedFingerprint != "" && currentFingerprint == "" {
		return []string{"current_fingerprint"}
	}
	if storedFingerprint != "" && storedFingerprint != currentFingerprint {
		return []string{"ai_slop_clean_stale"}
	}
	return nil
}
