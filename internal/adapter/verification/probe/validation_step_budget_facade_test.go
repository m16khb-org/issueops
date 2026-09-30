package probe

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/stepbudget"

type StepBudgetValidationDeps = stepbudget.StepBudgetValidationDeps

func ValidateStepBudgetBaselineWithDeps(binary, root string, seed int64, deps StepBudgetValidationDeps) selfverify.StepResult {
	return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, deps)
}

func StepBudgetBaselineSummaries(seed int64) (selfaugment.SelfAugmentSummary, selfaugment.SelfAugmentSummary) {
	return stepbudget.StepBudgetBaselineSummaries(seed)
}

func StepBudgetStateSnapshot(root string, seed int64, summary selfaugment.SelfAugmentSummary) selfaugment.SelfAugmentStateSnapshot {
	return stepbudget.StepBudgetStateSnapshot(root, seed, summary)
}

func StepBudgetValidationErrors(result selfaugment.SelfAugmentCompareResult) []string {
	return stepbudget.StepBudgetValidationErrors(result)
}
