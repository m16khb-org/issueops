package issueops

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
)

func ValidateReviewReflection(record model.IssueOpsRecord) error {
	review := record.DevilsAdvocateReview
	if review == nil || len(review.Findings) == 0 {
		return fmt.Errorf("no devil's-advocate findings to reflect")
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		return fmt.Errorf("cannot reflect findings before a linked issue")
	}
	return nil
}

func ValidateReviewReflectionStamp(record model.IssueOpsRecord) error {
	if record.DevilsAdvocateReview == nil {
		return fmt.Errorf("devil's-advocate review disappeared before reflect stamp")
	}
	return nil
}

// MarkReviewReflected applies a stamp after validating the current review and authority.
func MarkReviewReflected(record model.IssueOpsRecord, now string) model.IssueOpsRecord {
	review := *record.DevilsAdvocateReview
	review.IssueReflectedAt = now
	record.DevilsAdvocateReview = &review
	record.UpdatedAt = now
	return record
}

// PlanReviewRounds lists every recorded round in chronological order.
func PlanReviewRounds(review *model.IssueOpsDevilsAdvocateReview) []model.PlanReviewRound {
	rounds := make([]model.PlanReviewRound, 0, len(review.History)+1)
	for _, round := range review.History {
		rounds = append(rounds, model.PlanReviewRound{Verdict: round.Verdict, Findings: len(round.Findings)})
	}
	return append(rounds, model.PlanReviewRound{Verdict: review.Verdict, Findings: len(review.Findings)})
}
