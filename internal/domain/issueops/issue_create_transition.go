package issueops

import (
	"fmt"
	model "issueops/internal/contract/issueops"
)

func ValidateIssueCreateTransition(from, to string) error {
	allowed := false
	switch from {
	case model.IssueCreateIntentPending:
		allowed = to == model.IssueCreateIntentNotInvoked ||
			to == model.IssueCreateIntentInvokedUnknown ||
			to == model.IssueCreateIntentURLObserved ||
			to == model.IssueCreateIntentVerificationFailed ||
			to == model.IssueCreateIntentReceiptFailed ||
			to == model.IssueCreateIntentCompleted
	case model.IssueCreateIntentInvokedUnknown, model.IssueCreateIntentURLObserved:
		allowed = to == from ||
			to == model.IssueCreateIntentVerificationFailed ||
			to == model.IssueCreateIntentReceiptFailed ||
			to == model.IssueCreateIntentCompleted
	case model.IssueCreateIntentVerificationFailed, model.IssueCreateIntentReceiptFailed:
		allowed = to == from ||
			to == model.IssueCreateIntentVerificationFailed ||
			to == model.IssueCreateIntentReceiptFailed ||
			to == model.IssueCreateIntentCompleted
	}
	if !allowed {
		return fmt.Errorf("illegal issue create intent transition %s -> %s", from, to)
	}
	return nil
}
