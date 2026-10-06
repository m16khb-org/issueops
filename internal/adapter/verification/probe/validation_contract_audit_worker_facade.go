package probe

import (
	"issueops/internal/adapter/verification/probe/contractauditworker"
	selfverify "issueops/internal/contract/selfverify"
)

func ValidateCommandAudit(binary, root string, seed int64) selfverify.StepResult {
	return contractauditworker.ValidateCommandAudit(binary, root, seed)
}

func ValidateContractCheck(binary, root string) selfverify.StepResult {
	return contractauditworker.ValidateContractCheck(binary, root)
}

func ValidateToolConformance(binary, root string) selfverify.StepResult {
	return contractauditworker.ValidateToolConformance(binary, root)
}

func ValidateWorkerLifecycle(binary, root string, seed int64) selfverify.StepResult {
	return contractauditworker.ValidateWorkerLifecycle(binary, root, seed)
}
