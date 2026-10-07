package issueopsreview

import (
	"fmt"
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
)

const regressCap = 5

func NormalizeRegressionReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", fmt.Errorf("regression reason is required (the Brooks stop verdict)")
	}
	return reason, nil
}

// CheckRegression keeps the persisted eligibility checks ahead of child lookup.
func CheckRegression(snapshot reviewcontract.RegressionPreconditions) error {
	if snapshot.Phase != "plan" && snapshot.Phase != "compatibility-review" {
		return fmt.Errorf("design-review regression only applies from plan or compatibility-review phase, not %s", snapshot.Phase)
	}
	review := snapshot.Review
	if review != nil && review.Verdict == "revise" {
		return fmt.Errorf("devil's-advocate revise verdict must be resolved in place: update the linked plan, run and record a fresh devil's-advocate review, and proceed only after it passes or is explicitly waived; only a stop verdict may regress after remote reflection")
	}
	if review == nil || review.Verdict != "stop" {
		return fmt.Errorf("regress requires a recorded devil's-advocate stop verdict")
	}
	if strings.TrimSpace(review.IssueReflectedAt) == "" {
		return fmt.Errorf("reflect the devil's-advocate findings to the issue before regressing (issueops remote reflect-devils-advocate --confirm)")
	}
	if snapshot.RegressCount >= regressCap {
		return fmt.Errorf("regress cap reached: cycle %s already went through %d stop→re-plan rounds, so the plan is thrashing rather than converging; a human decision is required before any further automatic re-plan", snapshot.CycleID, snapshot.RegressCount)
	}
	return nil
}

func CheckRegressionChildren(activeChildIDs []string) error {
	if len(activeChildIDs) > 0 {
		return fmt.Errorf("children_active: %s", strings.Join(activeChildIDs, ", "))
	}
	return nil
}

func BuildRegressionChange(fromPhase, reason, at string) reviewcontract.RegressionChange {
	return reviewcontract.RegressionChange{
		FromPhase: fromPhase, ToPhase: "grill", EventReason: reason, At: at,
		DecisionTitle: "design-review devil's-advocate stop", DecisionBody: reason,
		DecisionKind: "scope", DecisionRationale: fmt.Sprintf("regressed from %s to grill for re-plan", fromPhase),
		StalePhases:         []string{"plan", "compatibility-review"},
		StaleNote:           fmt.Sprintf("stale: design-review regression (%s)", reason),
		ClearDesignApproval: true, ClearReview: true,
	}
}
