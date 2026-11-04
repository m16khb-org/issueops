package issueopsreview

import (
	"fmt"
	"strings"
)

// FeedbackRequiresIssueUpdate reports whether a contract_change feedback item still
// needs the issue updated, i.e. it has no issueUpdatedAt timestamp yet.
func FeedbackRequiresIssueUpdate(classification, issueUpdatedAt string) bool {
	return strings.EqualFold(strings.TrimSpace(classification), "contract_change") &&
		strings.TrimSpace(issueUpdatedAt) == ""
}

// FeedbackPhaseAfterAdd returns the cycle phase after feedback is added: feedback when
// clean evidence exists, otherwise the current phase. Adding feedback in the done
// phase is an error.
func FeedbackPhaseAfterAdd(phase string, hasCleanEvidence bool) (string, error) {
	if phase == "done" {
		return "", fmt.Errorf("cannot add feedback after %s phase", phase)
	}
	if hasCleanEvidence {
		return "feedback", nil
	}
	return phase, nil
}

// ValidateFeedbackIndex rejects a zero-based feedback index outside [0, count).
func ValidateFeedbackIndex(index, count int) error {
	if index < 0 || index >= count {
		// QA: an agent passed 1 for the only item; say indexes start at 0.
		return fmt.Errorf("feedback index %d out of range (have %d items; indexes start at 0)", index, count)
	}
	return nil
}
