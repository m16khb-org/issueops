package quality

import (
	contract "issueops/internal/contract/quality"
	quality "issueops/internal/domain/quality"
)

// InspectRequest selects the root to inspect and whether to compare the code SNR against
// the stored baseline (Trend) and/or overwrite that baseline (SaveBaseline).
type InspectRequest struct {
	Root         string
	Trend        bool
	SaveBaseline bool
}

// InspectBaselineDeps supplies the inspection and SNR baseline read/save functions that
// InspectWithBaseline composes.
type InspectBaselineDeps struct {
	Inspect         func(string) contract.InspectResult
	ReadSNRBaseline func(string) (float64, bool, error)
	SaveSNRBaseline func(string, float64) error
}

// InspectWithBaseline runs the inspection and then applies the SNR baseline workflow. It
// returns the result, the stored baseline ratio, and whether a baseline was present.
// Baseline read/save failures and SNR regressions are added to the result as findings
// instead of being returned as errors.
func InspectWithBaseline(request InspectRequest, deps InspectBaselineDeps) (contract.InspectResult, float64, bool) {
	result := deps.Inspect(request.Root)
	snrRatio, snrAvailable := quality.SuccessfulSignalValue(result.Signals, "code-snr")
	baseline, baselinePresent := float64(0), false
	if request.Trend {
		var baselineErr error
		baseline, baselinePresent, baselineErr = deps.ReadSNRBaseline(request.Root)
		if baselineErr != nil {
			quality.AddQualityCollectorFailure(&result, "read-baseline: "+baselineErr.Error())
		}
	}
	if request.SaveBaseline {
		if !snrAvailable {
			quality.AddQualityCollectorFailure(&result, "save-baseline: code-snr signal is unavailable")
		} else if err := deps.SaveSNRBaseline(request.Root, snrRatio); err != nil {
			quality.AddQualityCollectorFailure(&result, "save-baseline: "+err.Error())
		}
	}
	if request.Trend && snrAvailable && baselinePresent && quality.SNRRegressed(baseline, snrRatio) {
		quality.AddSNRRegressionFinding(&result, baseline, snrRatio)
	}
	return result, baseline, baselinePresent
}
