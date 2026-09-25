package issueopsreview

import (
	"strings"
	"testing"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

func TestRegressionPreconditionsPreserveFailureOrder(t *testing.T) {
	base := reviewcontract.RegressionPreconditions{
		CycleID: "io-1", Phase: "plan", Review: &reviewcontract.DevilsAdvocateReview{Verdict: "stop", IssueReflectedAt: "2026-09-24T00:00:00Z"},
	}
	if err := CheckRegression(base); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		set  func(*reviewcontract.RegressionPreconditions)
		want string
	}{
		{name: "phase first", set: func(s *reviewcontract.RegressionPreconditions) { s.Phase = "implement"; s.Review.Verdict = "revise" }, want: "only applies from plan"},
		{name: "revise before reflection", set: func(s *reviewcontract.RegressionPreconditions) {
			s.Review.Verdict = "revise"
			s.Review.IssueReflectedAt = ""
		}, want: "revise verdict must be resolved in place"},
		{name: "stop required", set: func(s *reviewcontract.RegressionPreconditions) { s.Review = nil }, want: "requires a recorded devil's-advocate stop"},
		{name: "reflection required", set: func(s *reviewcontract.RegressionPreconditions) { s.Review.IssueReflectedAt = "" }, want: "reflect the devil's-advocate findings"},
		{name: "cap", set: func(s *reviewcontract.RegressionPreconditions) { s.RegressCount = 3 }, want: "regress cap reached: cycle io-1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := base
			review := *base.Review
			snapshot.Review = &review
			test.set(&snapshot)
			if err := CheckRegression(snapshot); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRegressionReasonAndActiveChildren(t *testing.T) {
	if _, err := NormalizeRegressionReason("  "); err == nil || !strings.Contains(err.Error(), "reason is required") {
		t.Fatalf("blank reason accepted: %v", err)
	}
	if got, err := NormalizeRegressionReason("  re-plan  "); err != nil || got != "re-plan" {
		t.Fatalf("normalized reason = %q %v", got, err)
	}
	if err := CheckRegressionChildren([]string{"io-2", "io-3"}); err == nil || err.Error() != "children_active: io-2, io-3" {
		t.Fatalf("active children error = %v", err)
	}
}

func TestBuildRegressionChangePreservesAuditAndStaleInstructions(t *testing.T) {
	change := BuildRegressionChange("compatibility-review", "scope changed", "2026-09-24T00:00:00Z")
	if change.FromPhase != "compatibility-review" || change.ToPhase != "grill" ||
		change.EventReason != "scope changed" || change.DecisionKind != "scope" ||
		change.DecisionTitle != "design-review devil's-advocate stop" ||
		change.DecisionRationale != "regressed from compatibility-review to grill for re-plan" ||
		change.StaleNote != "stale: design-review regression (scope changed)" ||
		len(change.StalePhases) != 2 || change.StalePhases[0] != "plan" || change.StalePhases[1] != "compatibility-review" ||
		!change.ClearDesignApproval || !change.ClearReview {
		t.Fatalf("regression change = %+v", change)
	}
}
