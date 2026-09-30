package qualitycli

import (
	"fmt"
	outbound "issueops/internal/adapter/outbound/quality"
	application "issueops/internal/application/quality"
	contract "issueops/internal/contract/quality"
	quality "issueops/internal/domain/quality"
)

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

func testBaselineStore() application.SNRBaselineStore {
	return application.SNRBaselineStore{CanonicalRepository: outbound.CanonicalRepository, ReadState: hostDeps.StateRead, WriteState: hostDeps.StateWrite}
}
func readSNRBaseline(root string) (float64, bool, error) { return testBaselineStore().Read(root) }
func saveSNRBaseline(root string, ratio float64) error   { return testBaselineStore().Save(root, ratio) }
func snrBaselineIdentity(root string) (string, string, error) {
	repo, err := outbound.CanonicalRepository(root)
	if err != nil {
		return "", "", err
	}
	return repo, quality.SNRBaselineKey(repo), nil
}
