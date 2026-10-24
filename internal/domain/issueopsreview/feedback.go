package issueopsreview

import (
	"fmt"
	"strings"
)

func FeedbackRequiresIssueUpdate(classification, issueUpdatedAt string) bool {
	return strings.EqualFold(strings.TrimSpace(classification), "contract_change") &&
		strings.TrimSpace(issueUpdatedAt) == ""
}

func FeedbackPhaseAfterAdd(phase string, hasCleanEvidence bool) (string, error) {
	if phase == "done" {
		return "", fmt.Errorf("cannot add feedback after %s phase", phase)
	}
	if hasCleanEvidence {
		return "feedback", nil
	}
	return phase, nil
}

func ValidateFeedbackIndex(index, count int) error {
	if index < 0 || index >= count {
		// QA: an agent passed 1 for the only item; say indexes start at 0.
		return fmt.Errorf("feedback index %d out of range (have %d items; indexes start at 0)", index, count)
	}
	return nil
}
