// Package quality holds the pure quality-inspection rules: turning collector
// output (coverage, branch counts, audit items, pioneer evaluation, code
// signal-to-noise) into evidence, findings and gate statuses, and validating the
// signal-to-noise baseline. It performs no I/O; collectors live in adapters.
package quality

import (
	"crypto/sha256"
	"encoding/hex"
	contract "issueops/internal/contract/quality"
)

// SNRBaselineKey returns the state key under which the signal-to-noise baseline
// of repository is stored: a fixed prefix plus the first 8 bytes of the SHA-256
// of the repository identifier, hex encoded.
func SNRBaselineKey(repository string) string {
	sum := sha256.Sum256([]byte(repository))
	return "quality-snr-baseline-" + hex.EncodeToString(sum[:8])
}

// ValidSNRBaseline reports whether record is a usable baseline for repository:
// current schema version, same repository, and a ratio in [0, 1].
func ValidSNRBaseline(record contract.SNRBaselineRecord, repository string) bool {
	return record.SchemaVersion == contract.SNRBaselineSchemaVersion && record.Repository == repository && ValidSNRRatio(record.Ratio)
}
