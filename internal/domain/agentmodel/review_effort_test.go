package agentmodel

import "testing"

// 티어별 effort는 docs-only 하나만 낮춘다. 나머지는 host planner 기본값을 그대로
// 쓰므로 리뷰 강도가 조용히 떨어지지 않는다.
func TestIssueOpsReviewEffortForTierLowersOnlyDocsOnly(t *testing.T) {
	for _, host := range []string{"codex", "claude", "omo"} {
		_, planner, ok := PlannerDefaults(host)
		if !ok {
			t.Fatalf("%s must have planner defaults", host)
		}
		if got := ReviewEffortForTier(host, "docs-only"); got != ReviewEffortDocsOnly {
			t.Fatalf("%s docs-only effort = %q, want %q", host, got, ReviewEffortDocsOnly)
		}
		for _, tier := range []string{"contract", "schema-auth", "default", ""} {
			if got := ReviewEffortForTier(host, tier); got != planner {
				t.Fatalf("%s %q effort = %q, want the planner default %q", host, tier, got, planner)
			}
		}
	}
	if got := ReviewEffortForTier("unknown-host", "docs-only"); got != "" {
		t.Fatalf("an unknown host has no effort to lower, got %q", got)
	}
}
