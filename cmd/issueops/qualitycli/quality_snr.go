package qualitycli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	contract "issueops/internal/contract/quality"
	quality "issueops/internal/domain/quality"
	"path/filepath"
)

// snrEvidence renders the SNR signal's human-readable evidence lines.
func snrEvidence(snr SNRResult) []string { return quality.SNREvidence(snr) }

const snrBaselineSchemaVersion = 1

type snrBaselineRecord struct {
	SchemaVersion int     `json:"schema_version"`
	Repository    string  `json:"repository"`
	Ratio         float64 `json:"ratio"`
}

// SNRResult is a deterministic Shannon-style signal-to-noise measure over the
// repository's production Go source: signal lines (logic) versus noise lines
// (blank, comment-only, or structural-only such as a lone brace). It is a
// quantitative code-quality proxy — higher Ratio means less channel overhead.
// It does not judge whether the logic is correct, only its density.
type SNRResult = contract.SNRResult

// computeCodeSNR walks root for production (non-test) Go files and computes the
// signal-to-noise ratio. It is deterministic for a given file tree.
var snrScanner func(string) (SNRResult, error)

func ConfigureSNRScanner(scanner func(string) (SNRResult, error)) { snrScanner = scanner }

func computeCodeSNR(root string) (SNRResult, error) {
	if snrScanner == nil {
		return SNRResult{}, fmt.Errorf("quality SNR scanner is not configured")
	}
	return snrScanner(root)
}

// readSNRBaseline distinguishes an absent baseline from corrupted or
// unavailable state so --trend cannot silently suppress a regression.
func readSNRBaseline(root string) (float64, bool, error) {
	if hostDeps.StateRead == nil {
		return 0, false, fmt.Errorf("quality SNR baseline store is not configured")
	}
	repository, key, err := snrBaselineIdentity(root)
	if err != nil {
		return 0, false, err
	}
	res, err := hostDeps.StateRead(key)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !res.OK {
		return 0, false, fmt.Errorf("quality SNR baseline read returned no record")
	}
	var baseline snrBaselineRecord
	if err := json.Unmarshal([]byte(res.Record.Content), &baseline); err != nil {
		return 0, false, fmt.Errorf("parse quality SNR baseline: %w", err)
	}
	if baseline.SchemaVersion != snrBaselineSchemaVersion ||
		baseline.Repository != repository ||
		!validSNRRatio(baseline.Ratio) {
		return 0, false, fmt.Errorf("quality SNR baseline identity or ratio is invalid")
	}
	return baseline.Ratio, true, nil
}

// saveSNRBaseline persists the current ratio as the new baseline for trend
// comparison on a later run.
func saveSNRBaseline(root string, ratio float64) error {
	if hostDeps.StateWrite == nil {
		return fmt.Errorf("quality SNR baseline store is not configured")
	}
	if !validSNRRatio(ratio) {
		return fmt.Errorf("quality SNR baseline ratio must be finite and between zero and one")
	}
	repository, key, err := snrBaselineIdentity(root)
	if err != nil {
		return err
	}
	content, err := json.Marshal(snrBaselineRecord{
		SchemaVersion: snrBaselineSchemaVersion,
		Repository:    repository,
		Ratio:         ratio,
	})
	if err != nil {
		return err
	}
	_, err = hostDeps.StateWrite(key, string(content))
	return err
}

func snrBaselineIdentity(root string) (string, string, error) {
	repository, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve quality SNR repository: %w", err)
	}
	repository, err = filepath.EvalSymlinks(repository)
	if err != nil {
		return "", "", fmt.Errorf("resolve quality SNR repository symlinks: %w", err)
	}
	repository = filepath.Clean(repository)
	sum := sha256.Sum256([]byte(repository))
	return repository, "quality-snr-baseline-" + hex.EncodeToString(sum[:8]), nil
}

func validSNRRatio(ratio float64) bool {
	return quality.ValidSNRRatio(ratio)
}
