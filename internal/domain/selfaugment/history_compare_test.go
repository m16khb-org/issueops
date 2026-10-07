package selfaugment

import (
	"slices"
	"testing"

	contract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
)

func TestCompareSnapshotsSeparatesDurationContracts(t *testing.T) {
	for _, tc := range []struct {
		name             string
		baselineVersion  int
		candidateVersion int
		candidateHash    string
		compareDurations bool
	}{
		{"matching", 7, 7, "same", true},
		{"older to current", 6, 7, "same", false},
		{"current to older", 7, 6, "same", false},
		{"different hash", 7, 7, "different", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			baseline := contract.SelfAugmentStateSnapshot{
				OK: true, ElapsedMS: 1000,
				Summary: contract.SelfAugmentSummary{
					Contract: verifycontract.SelfVerificationContract{
						Name: "self_verification_summary", Version: tc.baselineVersion, Hash: "same",
					},
					TotalSteps: 1, PassedSteps: 1, StepLabels: []string{"go test"},
					SlowestSteps: []contract.SelfAugmentSlowStep{{Label: "go test", DurationMS: 100}},
					StepDurationStats: []contract.SelfAugmentStepDurationStat{
						{Label: "go test", Count: 1, P95DurationMS: 100},
					},
				},
			}
			candidate := baseline
			candidate.Summary.Contract.Version = tc.candidateVersion
			candidate.Summary.Contract.Hash = tc.candidateHash
			candidate.Summary.PassedSteps = 0
			candidate.Summary.FailedSteps = 1
			candidate.Summary.SlowestSteps = []contract.SelfAugmentSlowStep{{Label: "go test", DurationMS: 500}}
			candidate.Summary.StepDurationStats = []contract.SelfAugmentStepDurationStat{
				{Label: "go test", Count: 1, P95DurationMS: 500},
			}

			got := CompareSnapshots("baseline", "candidate", 10, baseline, candidate, "fixture")
			if (len(got.SlowStepRegressions) > 0) != tc.compareDurations ||
				(len(got.StepBudgetRegressions) > 0) != tc.compareDurations {
				t.Fatalf("duration comparison crossed contract boundary: %+v", got)
			}
			if slices.Contains(got.Warnings, "step_duration_contract_mismatch") == tc.compareDurations {
				t.Fatalf("contract warning does not match comparison coverage: %v", got.Warnings)
			}
			if !slices.Contains(got.Regressions, "failed_steps_increased_by_1") {
				t.Fatalf("non-duration regression was lost: %v", got.Regressions)
			}
		})
	}
}
