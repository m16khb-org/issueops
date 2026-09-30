package selfaugment

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/selfaugment"
)

func PreparePromotion(fromKey, baselineKey string, confirm bool, stateDir string) (contract.SelfAugmentPromoteResult, error) {
	result := contract.SelfAugmentPromoteResult{
		OK:          false,
		StateDir:    stateDir,
		FromKey:     fromKey,
		BaselineKey: baselineKey,
		Confirm:     confirm,
		DryRun:      !confirm,
	}
	if strings.TrimSpace(fromKey) == "" {
		return result, fmt.Errorf("from-key is required")
	}
	if strings.TrimSpace(baselineKey) == "" {
		return result, fmt.Errorf("baseline-key is required")
	}
	return result, nil
}

func ObservePromotionSource(result contract.SelfAugmentPromoteResult, snapshot contract.SelfAugmentStateSnapshot, allowFailedSource bool) (contract.SelfAugmentPromoteResult, error) {
	result.SnapshotGeneratedAt = snapshot.GeneratedAt
	result.Summary = snapshot.Summary
	// Gate semantics (measured by the SV-B case): a baseline is the reference
	// every future compare regresses against, so promoting a failed run
	// silently poisons all later judgements. Refuse unless explicitly
	// overridden; dry-run stays diagnostic and just reports the flag.
	result.SourcePassed = snapshot.OK && snapshot.Summary.TerminationEligible
	if !result.Confirm {
		result.OK = true
		return result, nil
	}
	if !result.SourcePassed && !allowFailedSource {
		return result, fmt.Errorf(
			"refusing to promote: source snapshot %q did not pass the gate (ok=%v, termination_eligible=%v); rerun self-verify or pass --allow-failed-source",
			result.FromKey, snapshot.OK, snapshot.Summary.TerminationEligible)
	}
	return result, nil
}
