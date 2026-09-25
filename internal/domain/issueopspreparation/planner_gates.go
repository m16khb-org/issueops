package issueopspreparation

import (
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

// MissingPlannerGateKeys는 Orca owner를 띄우기 전에 반드시 기록돼 있어야 하는
// planner 소유 전제 중 빠진 것을 돌려준다(#319).
//
// 이 검사가 필요한 이유는 owner가 이것들을 보충할 수 없기 때문이다.
// intent contract, design review, devil's advocate review는 planner의 판단이며
// owner packet의 commands map에 없다. 그런데 owner의 compatibility review와
// implement 진입 게이트는 이것들을 요구한다. 없는 상태로 띄우면 owner는
// claim까지 완주한 뒤 채울 수 없는 게이트에 부딪혀 반드시 실패한다.
//
// 실측: lifecycle io-cb83a79e1bfd에서 owner가 claim, await-link,
// link-verified까지 완주한 뒤 "missing design_review, intent_contract"로
// fail-closed 했고, 그 셋이 planner 소유라 보충할 수 없다고 보고했다.
//
// prepare가 이것을 미리 막으면 owner를 헛되이 띄우지 않고, coordinator는
// 무엇을 먼저 해야 하는지 정확한 명령으로 알게 된다.
func MissingPlannerGateKeys(evidence preparationcontract.PlannerEvidence) []string {
	var missing []string
	if !plannerIntentRecorded(evidence.Intent) {
		missing = append(missing, "intent_contract")
	}
	if !plannerDesignReviewApproved(evidence.DesignReview) {
		missing = append(missing, "design_review")
	}
	if !plannerDevilsAdvocateCleared(evidence.DevilsAdvocate) {
		missing = append(missing, "devils_advocate_review")
	}
	return missing
}

func plannerIntentRecorded(intent preparationcontract.PlannerIntentEvidence) bool {
	return strings.TrimSpace(intent.RawRequest) != "" &&
		strings.TrimSpace(intent.InterpretedIntent) != "" &&
		len(nonEmptyValues(intent.SuccessCriteria)) > 0
}

func plannerDesignReviewApproved(review preparationcontract.PlannerDesignEvidence) bool {
	return review.Approved && strings.TrimSpace(review.ProblemSummary) != "" &&
		strings.TrimSpace(review.ProposedDesign) != "" && len(nonEmptyValues(review.Verification)) > 0
}

// plannerDevilsAdvocateCleared는 review가 기록됐고, stop/revise 판정이면
// 명시적으로 waive됐는지 본다. adapter의 implement-entry 게이트와 같은 규칙이다.
// 판정은 검토한 플랜 내용에 묶여 있어야(reviewed_plan_digest) 한다 — 여기는
// fs가 없으므로 digest 존재만 보고, 현재 플랜과의 비교는 adapter(implement
// 진입·owner preflight)가 한다. delegation이 합성한 자식 판정
// (reviewer_pattern delegated-parent-review)은 부모 판정을 상속하므로 면제다.
func plannerDevilsAdvocateCleared(review preparationcontract.PlannerDevilsAdvocateEvidence) bool {
	if strings.TrimSpace(review.RecordedAt) == "" {
		return false
	}
	verdict := strings.TrimSpace(review.Verdict)
	if (verdict == "stop" || verdict == "revise") && !review.Waived {
		return false
	}
	if review.ReviewerPattern != "delegated-parent-review" && strings.TrimSpace(review.ReviewedPlanDigest) == "" {
		return false
	}
	return true
}

func nonEmptyValues(values []string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
