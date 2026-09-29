package quality

import (
	contract "issueops/internal/contract/quality"
	quality "issueops/internal/domain/quality"
)

type InspectRequest struct {
	Root         string
	Trend        bool
	SaveBaseline bool
}
type InspectBaselineDeps struct {
	Inspect         func(string) contract.InspectResult
	ReadSNRBaseline func(string) (float64, bool, error)
	SaveSNRBaseline func(string, float64) error
}

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
