package issueopspublication

import (
	"fmt"
	publicationcontract "issueops/internal/contract/issueopspublication"
)

func ValidateIssueCreateTransition(from, to string) error {
	allowed := false
	switch from {
	case publicationcontract.IssueCreateIntentPending:
		allowed = to == publicationcontract.IssueCreateIntentNotInvoked ||
			to == publicationcontract.IssueCreateIntentInvokedUnknown ||
			to == publicationcontract.IssueCreateIntentURLObserved ||
			to == publicationcontract.IssueCreateIntentVerificationFailed ||
			to == publicationcontract.IssueCreateIntentReceiptFailed ||
			to == publicationcontract.IssueCreateIntentCompleted
	case publicationcontract.IssueCreateIntentInvokedUnknown, publicationcontract.IssueCreateIntentURLObserved:
		allowed = to == from ||
			to == publicationcontract.IssueCreateIntentVerificationFailed ||
			to == publicationcontract.IssueCreateIntentReceiptFailed ||
			to == publicationcontract.IssueCreateIntentCompleted
	case publicationcontract.IssueCreateIntentVerificationFailed, publicationcontract.IssueCreateIntentReceiptFailed:
		allowed = to == from ||
			to == publicationcontract.IssueCreateIntentVerificationFailed ||
			to == publicationcontract.IssueCreateIntentReceiptFailed ||
			to == publicationcontract.IssueCreateIntentCompleted
	}
	if !allowed {
		return fmt.Errorf("illegal issue create intent transition %s -> %s", from, to)
	}
	return nil
}
