package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// AISlopCleanFingerprintReadinessMissing compares the fingerprint stored by
// ai-slop-clean with currentFingerprint and returns the gate that failed: a
// missing stored or current fingerprint, or a stale one. It returns nil when
// no clean was recorded or the fingerprints agree.
func AISlopCleanFingerprintReadinessMissing(record model.Record, currentFingerprint string) []string {
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
