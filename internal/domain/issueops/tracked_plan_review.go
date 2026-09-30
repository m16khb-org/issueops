package issueops

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
)

// RenderTrackedPlanReview writes every plan-review round with its findings
// for readers who want more than the issue's one-line summary. Hashes and
// local paths in findings are masked the same way the issue region masks them.
func RenderTrackedPlanReview(review *issueops.IssueOpsDevilsAdvocateReview) string {
	var b strings.Builder
	b.WriteString("# 계획 검토 기록\n")
	rounds := append(append([]issueops.IssueOpsDevilsAdvocateRound{}, review.History...), issueops.IssueOpsDevilsAdvocateRound{
		Verdict: review.Verdict, Findings: review.Findings, Waived: review.Waived, WaiverRationale: review.WaiverRationale,
	})
	for i, round := range rounds {
		fmt.Fprintf(&b, "\n## %d차: %s\n", i+1, planReviewVerdictLabel(round.Verdict))
		if round.Waived {
			fmt.Fprintf(&b, "\n생략: %s\n", maskHarnessValues(strings.TrimSpace(round.WaiverRationale)))
		}
		if len(round.Findings) == 0 {
			continue
		}
		b.WriteString("\n")
		for _, finding := range round.Findings {
			if finding = strings.TrimSpace(finding); finding != "" {
				fmt.Fprintf(&b, "- %s\n", maskHarnessValues(finding))
			}
		}
	}
	return b.String()
}
