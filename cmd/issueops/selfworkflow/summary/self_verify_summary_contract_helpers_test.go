package summary

import domain "issueops/internal/domain/selfverify"

func SelfVerificationContractValue() SelfVerificationContract { return domain.ContractValue() }
func SelfVerificationGoalDefinitions() []SelfVerificationGoalDefinition {
	return domain.GoalDefinitions()
}
func SelfVerificationCoverageDefinitions() []SelfVerificationCoverageDefinition {
	return domain.CoverageDefinitions()
}
func SelfVerificationCoverageForLabels(labels []string) ([]SelfVerificationCoverage, []string) {
	return domain.CoverageForLabels(labels)
}
