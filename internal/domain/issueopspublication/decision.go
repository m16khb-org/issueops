// Package issueopspublication 패키지는 원격 PR/MR 생성 게시의 순수 규칙을 담는다.
// 생성 자격, 기록 직전 권한, 후보 대조, 재시도와 조정 결정, 의도 상태 전이를 판단하며 I/O는 하지 않는다.
// domain 계층에 속하고 contract 계층의 타입만 사용한다.
package issueopspublication

import (
	"fmt"

	contract "issueops/internal/contract/issueopspublication"
)

type action string

// 조정 결정 동작: ActionAdopt는 후보를 채택하고, ActionRetry는 생성을 다시 시도하며, ActionPreserve는 의도를 그대로 보존한다.
const (
	ActionAdopt    action = "adopt"
	ActionRetry    action = "retry"
	ActionPreserve action = "preserve"
)

// ReconcileFacts 타입은 조정 결정에 쓰는 사실이다. 후보 수, 후보 없음이 권위 있는 결과인지,
// 호출 여부, 재시도 횟수, 그리고 호출 결과를 더는 바꿀 수 없다고 확정됐는지를 담는다.
type ReconcileFacts struct {
	CandidateCount    int
	AuthoritativeZero bool
	Invocation        contract.InvocationState
	RetryCount        int
	// CrashSettled says an unknown invocation can no longer land: the create
	// is older than the quiet period and no provider create or issueops
	// create-pr for this head is alive. A caller that died inside the create
	// otherwise left the intent with no exit but abandon (QA, killed process).
	CrashSettled bool
}

// Decision 타입은 조정 결정 결과다. 선택한 동작, 채택할 후보의 인덱스, 보존하는 이유를 담는다.
type Decision struct {
	Action         action
	CandidateIndex int
	Reason         string
}

// DecideReconcile 함수는 facts로 후보를 채택할지, 재시도할지, 보존할지 정한다.
// 후보가 하나면 채택한다. 후보가 없을 때는 권위 있는 0건이고, 호출하지 않았음이 증명되었거나 호출 결과가 확정되었으며,
// 재시도 횟수가 0일 때만 재시도하고 나머지는 이유와 함께 보존한다. 후보 수가 음수이면 오류를 반환한다.
func DecideReconcile(facts ReconcileFacts) (Decision, error) {
	if facts.CandidateCount < 0 {
		return Decision{}, fmt.Errorf("candidate count must not be negative")
	}
	if facts.CandidateCount > 1 {
		return Decision{Action: ActionPreserve, Reason: "multiple-candidates"}, nil
	}
	if facts.CandidateCount == 1 {
		return Decision{Action: ActionAdopt, CandidateIndex: 0}, nil
	}
	if !facts.AuthoritativeZero {
		return Decision{Action: ActionPreserve, Reason: "non-authoritative-zero"}, nil
	}
	if facts.Invocation != contract.InvocationNotInvokedProven && !facts.CrashSettled {
		return Decision{Action: ActionPreserve, Reason: "unknown-invocation"}, nil
	}
	if facts.RetryCount != 0 {
		return Decision{Action: ActionPreserve, Reason: "retry-exhausted"}, nil
	}
	return Decision{Action: ActionRetry}, nil
}

// EligibilityReason 타입은 생성 자격 검사에서 실패한 항목의 이름이다.
type EligibilityReason string

// ValidateCreateEligibility 함수는 생성 자격 사실이 모두 충족되는지 검사한다.
// provider와 kind 조합, PR 단계, 산출물 없음, 브랜치 권한, 라벨과 담당자 표준화를 확인하고,
// 확정 실행이면 실행 활성 여부와 대기 작업 없음도 확인한다. 실패하면 실패 항목과 오류를 반환한다.
func ValidateCreateEligibility(facts contract.CreateEligibility) (EligibilityReason, error) {
	if reason, err := validateEligibilityTarget(facts); err != nil {
		return reason, err
	}
	checks := []struct {
		ok     bool
		reason EligibilityReason
	}{
		{ok: facts.PhasePR, reason: "phase"},
		{ok: facts.NoArtifact, reason: "artifact"},
		{ok: facts.BranchAuthority, reason: "branch-authority"},
		{ok: facts.CanonicalLabelsAssignees, reason: "labels-assignees"},
	}
	for _, check := range checks {
		if !check.ok {
			return check.reason, fmt.Errorf("publication eligibility: %s", check.reason)
		}
	}
	return validateConfirmEligibility(facts)
}

// validateEligibilityTarget은 provider가 지원되는 값이고 kind가 provider와 맞는지 검사한다.
func validateEligibilityTarget(facts contract.CreateEligibility) (EligibilityReason, error) {
	if facts.Provider != "github" && facts.Provider != "gitlab" {
		return "provider", fmt.Errorf("publication eligibility: provider")
	}
	if (facts.Provider == "github" && facts.Kind != "pr") ||
		(facts.Provider == "gitlab" && facts.Kind != "mr") {
		return "kind", fmt.Errorf("publication eligibility: kind")
	}
	return "", nil
}

// validateConfirmEligibility는 확정 실행일 때만 실행 활성 상태와 대기 작업 없음을 검사한다.
func validateConfirmEligibility(facts contract.CreateEligibility) (EligibilityReason, error) {
	if facts.Confirm && !facts.ExecutionActive {
		return "execution", fmt.Errorf("publication eligibility: execution")
	}
	if facts.Confirm && !facts.NoPending {
		return "pending", fmt.Errorf("publication eligibility: pending")
	}
	return "", nil
}

// RetryOutcomeFacts 타입은 재시도 호출 결과를 분류하는 사실이다. 호출 상태와 호출 실패 여부를 담는다.
type RetryOutcomeFacts struct {
	Invocation contract.InvocationState
	CallFailed bool
}

// RetryOutcome 타입은 재시도 호출 뒤 의도를 어떻게 처리할지 나타낸다.
type RetryOutcome string

// 재시도 결과: RetryOutcomePreserve는 의도를 보존하고, RetryOutcomeTerminalNotInvoked는 호출하지 않았음이 증명된 채 종료한다.
const (
	retryOutcomeVerify             RetryOutcome = "verify"
	RetryOutcomePreserve           RetryOutcome = "preserve"
	RetryOutcomeTerminalNotInvoked RetryOutcome = "terminal-not-invoked"
)

// DecideRetryOutcome 함수는 재시도 호출 결과에 따른 처리를 정한다.
// 호출이 실패하지 않았으면 검증으로 진행하고, 실패했으면 호출하지 않았음이 증명된 경우에만 종료하며 그 밖에는 보존한다.
func DecideRetryOutcome(facts RetryOutcomeFacts) RetryOutcome {
	if !facts.CallFailed {
		return retryOutcomeVerify
	}
	if facts.Invocation == contract.InvocationNotInvokedProven {
		return RetryOutcomeTerminalNotInvoked
	}
	return RetryOutcomePreserve
}
