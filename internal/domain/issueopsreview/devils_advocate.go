// Package issueopsreview holds the pure domain rules for IssueOps review gates:
// architecture, compatibility, devil's-advocate, implementation, project docs, and
// schema reviews, plus feedback completion and regression. It validates review
// requests, builds records, and reports missing gate keys without any I/O; callers
// in the application layer supply the observed state.
package issueopsreview

import (
	"fmt"
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	"issueops/internal/domain/policy"
)

const reviseRoundCap = 5

// ValidateReview decides whether the supplied evidence supports its verdict.
// The caller supplies recordedAt so this rule has no clock dependency.
func ValidateReview(req reviewcontract.DevilsAdvocateReviewRequest, recordedAt string) (reviewcontract.DevilsAdvocateReview, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	if verdict != "pass" && verdict != "revise" && verdict != "stop" {
		return reviewcontract.DevilsAdvocateReview{}, fmt.Errorf("verdict must be pass, revise, or stop")
	}
	reviewerContext := strings.ToLower(strings.TrimSpace(req.ReviewerContext))
	if reviewerContext != "subagent" && reviewerContext != "inline" {
		return reviewcontract.DevilsAdvocateReview{}, fmt.Errorf("reviewer_context must be subagent or inline")
	}
	findings := cleanReviewValues(req.Findings)
	rationale := strings.TrimSpace(req.WaiverRationale)
	if err := validateReviewFindings(verdict, req.Waived, rationale, findings); err != nil {
		return reviewcontract.DevilsAdvocateReview{}, err
	}
	return reviewcontract.DevilsAdvocateReview{
		Verdict: verdict, Findings: findings, Waived: req.Waived,
		WaiverRationale: policy.RedactFreeform(rationale), ReviewerPattern: "devils-advocate-review",
		ReviewerContext: reviewerContext, RecordedAt: recordedAt,
		ReviewerHost:   strings.ToLower(strings.TrimSpace(req.ReviewerHost)),
		ReviewerModel:  strings.TrimSpace(req.ReviewerModel),
		ReviewerEffort: strings.TrimSpace(req.ReviewerEffort),
	}, nil
}

func validateReviewFindings(verdict string, waived bool, rationale string, findings []string) error {
	if waived && rationale == "" {
		return fmt.Errorf("waived review requires waiver_rationale")
	}
	if verdict == "pass" && len(findings) == 0 {
		return fmt.Errorf("pass verdict requires at least one finding (what was attacked and why it failed)")
	}
	if (verdict == "stop" || verdict == "revise") && !waived && len(findings) == 0 {
		return fmt.Errorf("%s verdict requires findings or an explicit waiver", verdict)
	}
	return nil
}

func cleanReviewValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		value = policy.RedactFreeform(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// ReviseRoundCapError is returned by ApplyReview when the plan phase already holds the
// maximum number of unwaived revise rounds. Count is the number found.
type ReviseRoundCapError struct{ Count int }

func (e *ReviseRoundCapError) Error() string { return "revise round cap reached" }

// ApplyReview preserves prior rounds oldest-first and rejects a sixth
// unwaived revise in the current plan phase.
func ApplyReview(previous *reviewcontract.DevilsAdvocateReview, next reviewcontract.DevilsAdvocateReview) (reviewcontract.DevilsAdvocateReview, error) {
	if previous == nil {
		return next, nil
	}
	priorRounds := append(append([]reviewcontract.DevilsAdvocateRound{}, previous.History...), roundOf(*previous))
	if next.Verdict == "revise" && !next.Waived {
		unwaived := 0
		for _, round := range priorRounds {
			if round.Verdict == "revise" && !round.Waived {
				unwaived++
			}
		}
		if unwaived >= reviseRoundCap {
			return reviewcontract.DevilsAdvocateReview{}, &ReviseRoundCapError{Count: unwaived}
		}
	}
	next.History = priorRounds
	return next, nil
}

func roundOf(review reviewcontract.DevilsAdvocateReview) reviewcontract.DevilsAdvocateRound {
	return reviewcontract.DevilsAdvocateRound{
		Verdict: review.Verdict, Findings: review.Findings, Waived: review.Waived,
		WaiverRationale: review.WaiverRationale, ReviewerContext: review.ReviewerContext,
		ReviewerHost: review.ReviewerHost, ReviewerModel: review.ReviewerModel, ReviewerEffort: review.ReviewerEffort,
		ReviewedPlanDigest: review.ReviewedPlanDigest, RecordedAt: review.RecordedAt,
	}
}
