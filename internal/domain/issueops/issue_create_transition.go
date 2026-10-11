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
			issueCreateSettledIntent(to)
	case model.IssueCreateIntentInvokedUnknown, model.IssueCreateIntentURLObserved,
		model.IssueCreateIntentVerificationFailed, model.IssueCreateIntentReceiptFailed:
		allowed = to == from || issueCreateSettledIntent(to)
	}
	if !allowed {
		return fmt.Errorf("illegal issue create intent transition %s -> %s", from, to)
	}
	return nil
}

// issueCreateSettledIntent는 pending과 invoked 계열 상태에서 공통으로 이동할 수 있는 검증 실패·영수증 실패·완료 상태인지 판정한다.
func issueCreateSettledIntent(intent string) bool {
	return intent == model.IssueCreateIntentVerificationFailed ||
		intent == model.IssueCreateIntentReceiptFailed ||
		intent == model.IssueCreateIntentCompleted
}
