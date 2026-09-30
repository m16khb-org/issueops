package quality

import (
	"crypto/sha256"
	"encoding/hex"
	contract "issueops/internal/contract/quality"
)

func SNRBaselineKey(repository string) string {
	sum := sha256.Sum256([]byte(repository))
	return "quality-snr-baseline-" + hex.EncodeToString(sum[:8])
}
func ValidSNRBaseline(record contract.SNRBaselineRecord, repository string) bool {
	return record.SchemaVersion == contract.SNRBaselineSchemaVersion && record.Repository == repository && ValidSNRRatio(record.Ratio)
}
