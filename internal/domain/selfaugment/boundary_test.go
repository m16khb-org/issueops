package selfaugment

import (
	"math"
	"slices"
	"strings"
	"testing"

	contract "issueops/internal/contract/selfaugment"
)

func TestCandidateScoreCombinesAllWeightedTerms(t *testing.T) {
	got := CandidateScore(contract.SelfAugmentCandidate{Impact: 10, Feasibility: 20, Novelty: 30, Risk: 40})
	// 3.8 + 6.0 + 6.0 + 7.2
	if math.Abs(got-23) > 1e-9 {
		t.Fatalf("score=%v want 23", got)
	}
	if got := CandidateScore(contract.SelfAugmentCandidate{Feasibility: 100, Risk: 100}); math.Abs(got-30) > 1e-9 {
		t.Fatalf("feasibility weight 0.30 expected 30, got %v", got)
	}
	if got := CandidateScore(contract.SelfAugmentCandidate{Novelty: 100, Risk: 100}); math.Abs(got-20) > 1e-9 {
		t.Fatalf("novelty weight 0.20 expected 20, got %v", got)
	}
}

func TestCompareSnapshotsReportsStepDeltasAndWarnings(t *testing.T) {
	baseline := contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 100, Summary: contract.SelfAugmentSummary{
		TotalSteps: 10, FailedSteps: 1, FailureCause: "cause-a", StepLabels: []string{"a"},
	}}
	candidate := contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 100, Summary: contract.SelfAugmentSummary{
		TotalSteps: 7, FailedSteps: 4, FailureCause: "cause-b", StepLabels: []string{"a"},
	}}
	result := CompareSnapshots("b", "c", 10, baseline, candidate, "/d")
	if result.FailedStepsDelta != 3 || result.TotalStepsDelta != -3 {
		t.Fatalf("deltas failed=%d total=%d", result.FailedStepsDelta, result.TotalStepsDelta)
	}
	for _, want := range []string{"failure_cause_changed:cause-a->cause-b", "total_steps_delta_-3"} {
		if !slices.Contains(result.Warnings, want) {
			t.Fatalf("missing warning %q in %v", want, result.Warnings)
		}
	}

	// Cause changes are only reported when both snapshots failed.
	for _, tc := range []struct {
		name                 string
		baseFailed, candFail int
	}{{"baseline passed", 0, 2}, {"candidate passed", 2, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			b, c := baseline, candidate
			b.Summary.FailedSteps, c.Summary.FailedSteps = tc.baseFailed, tc.candFail
			b.Summary.TotalSteps, c.Summary.TotalSteps = 5, 5
			result := CompareSnapshots("b", "c", 10, b, c, "/d")
			for _, w := range result.Warnings {
				if strings.HasPrefix(w, "failure_cause_changed") {
					t.Fatalf("unexpected %q", w)
				}
			}
			if slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.HasPrefix(w, "total_steps_delta") }) {
				t.Fatalf("equal totals must not warn: %v", result.Warnings)
			}
		})
	}

	same := baseline
	same.Summary.FailedSteps = 2
	other := same
	if got := CompareSnapshots("b", "c", 10, same, other, "/d").Warnings; len(got) != 0 {
		t.Fatalf("same failure cause must not warn: %v", got)
	}
}

func TestCompareSnapshotsElapsedBaselineAndThresholdBoundaries(t *testing.T) {
	zero := CompareSnapshots("b", "c", 10,
		contract.SelfAugmentStateSnapshot{OK: true},
		contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 50}, "/d")
	if !slices.Contains(zero.Warnings, "baseline_elapsed_zero") || zero.ElapsedDeltaPct != 0 || zero.ElapsedDeltaMS != 50 {
		t.Fatalf("zero baseline: %+v", zero)
	}
	if zero.Regressed {
		t.Fatalf("zero baseline must not regress: %v", zero.Regressions)
	}

	atLimit := CompareSnapshots("b", "c", 50,
		contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 100},
		contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 150}, "/d")
	if atLimit.ElapsedDeltaPct != 50 || atLimit.Regressed {
		t.Fatalf("increase equal to the limit is allowed: %+v", atLimit)
	}
	over := CompareSnapshots("b", "c", 49.99,
		contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 100},
		contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 150}, "/d")
	if !slices.Contains(over.Regressions, "elapsed_ms_increased_by_50.00_pct") {
		t.Fatalf("regressions=%v", over.Regressions)
	}
}

func TestSlowStepRegressionThresholdsAreExact(t *testing.T) {
	base := []contract.SelfAugmentSlowStep{{Label: "z", DurationMS: 0}, {Label: "s", DurationMS: 100}}
	cand := []contract.SelfAugmentSlowStep{{Label: "z", DurationMS: 10}, {Label: "s", DurationMS: 150}}
	if got := CompareSlowestStepRegressions(base, cand, 50); len(got) != 0 {
		t.Fatalf("increase equal to limit must pass and zero baseline is skipped: %+v", got)
	}
	got := CompareSlowestStepRegressions(base, cand, 49)
	if len(got) != 1 || got[0].Label != "s" || got[0].DeltaPct != 50 || got[0].BaselineDurationMS != 100 || got[0].CandidateDurationMS != 150 {
		t.Fatalf("got=%+v", got)
	}
}

func TestStepBudgetRegressionThresholdsAreExact(t *testing.T) {
	stat := func(label string, p95 int64) contract.SelfAugmentStepDurationStat {
		return contract.SelfAugmentStepDurationStat{Label: label, P95DurationMS: p95}
	}
	// delta of exactly the floor (25ms) regresses; one ms less is ignored.
	got := CompareStepBudgetRegressions(
		[]contract.SelfAugmentStepDurationStat{stat("floor", 100), stat("below", 100), stat("pct", 100)},
		[]contract.SelfAugmentStepDurationStat{stat("floor", 100+stepBudgetMinRegressionMS), stat("below", 100+stepBudgetMinRegressionMS-1), stat("pct", 150)}, 10)
	if len(got) != 2 || got[0].Label != "pct" || got[1].Label != "floor" {
		t.Fatalf("got=%+v", got)
	}
	if got[0].DeltaPct != 50 || got[1].DeltaPct != 25 {
		t.Fatalf("delta pct=%v,%v want 50,25", got[0].DeltaPct, got[1].DeltaPct)
	}

	// increase equal to the percentage limit is allowed.
	atLimit := CompareStepBudgetRegressions(
		[]contract.SelfAugmentStepDurationStat{stat("a", 100)},
		[]contract.SelfAugmentStepDurationStat{stat("a", 150)}, 50)
	if len(atLimit) != 0 {
		t.Fatalf("atLimit=%+v", atLimit)
	}
}

func TestPlanHistoryRetentionWithinBudgetHasNoPruneRecommendation(t *testing.T) {
	decision := PlanHistoryRetention([]string{"a", "b"}, 2, true, true)
	if decision.Recommendation != "within_retention_budget" || decision.Warning != "" || len(decision.CandidateKeys) != 0 {
		t.Fatalf("decision=%+v", decision)
	}
	if !decision.Apply || decision.DryRun {
		t.Fatalf("confirmed prune must apply: %+v", decision)
	}
}

func TestPrepareLessonDefaultsAndPreservesExplicitSourceAndSeverity(t *testing.T) {
	defaults, err := PrepareLesson(contract.SelfAugmentLessonRequest{Lesson: "l", NextAction: "n"}, "/r", "cand")
	if err != nil || defaults.Source != "self-augment" || defaults.Severity != "info" {
		t.Fatalf("defaults=%+v err=%v", defaults, err)
	}
	explicit, err := PrepareLesson(contract.SelfAugmentLessonRequest{Lesson: "l", NextAction: "n", Source: " dogfood ", Severity: " high "}, "/r", "cand")
	if err != nil || explicit.Source != "dogfood" || explicit.Severity != "high" {
		t.Fatalf("explicit=%+v err=%v", explicit, err)
	}
}

func TestStateKeySlugKeepsAlphanumericRangeEdges(t *testing.T) {
	for in, want := range map[string]string{
		"az09":     "az09",
		"AZ09":     "az09",
		"`a{z/0:9": "a-z-0-9",
		"@a[":      "a",
	} {
		if got := StateKeySlug(in); got != want {
			t.Errorf("StateKeySlug(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSummarizeStepsOrdersSlowestTiesByIterationThenLabel(t *testing.T) {
	summary := SummarizeSteps([]SummaryRun{
		{Iteration: 2, Steps: []SummaryStep{{Label: "a", OK: true, DurationMS: 10}}},
		{Iteration: 1, Steps: []SummaryStep{{Label: "b", OK: true, DurationMS: 10}, {Label: "a", OK: true, DurationMS: 10}, {Label: "c", OK: true, DurationMS: 20}}},
	}, 95)
	var got []string
	for _, s := range summary.SlowestSteps {
		got = append(got, s.Label+string(rune('0'+s.Iteration)))
	}
	if want := []string{"c1", "a1", "b1", "a2"}; !slices.Equal(got, want) {
		t.Fatalf("order=%v want %v", got, want)
	}
}

func TestCompareSnapshotsZeroElapsedOnBothSidesDoesNotWarn(t *testing.T) {
	result := CompareSnapshots("b", "c", 10,
		contract.SelfAugmentStateSnapshot{OK: true},
		contract.SelfAugmentStateSnapshot{OK: true}, "/d")
	if slices.Contains(result.Warnings, "baseline_elapsed_zero") {
		t.Fatalf("warnings=%v", result.Warnings)
	}
}
