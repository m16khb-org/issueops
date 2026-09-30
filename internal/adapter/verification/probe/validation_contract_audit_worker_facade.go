package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

import "issueops/internal/adapter/verification/probe/contractauditworker"

func ValidateCommandAudit(binary, root string, seed int64) selfverify.StepResult {
	return contractauditworker.ValidateCommandAudit(binary, root, seed)
}

func ValidateCommandAuditWithDeps(binary, root string, seed int64, deps contractauditworker.ValidationDeps) selfverify.StepResult {
	return contractauditworker.ValidateCommandAuditWithDeps(binary, root, seed, deps)
}

func ValidateContractCheck(binary, root string) selfverify.StepResult {
	return contractauditworker.ValidateContractCheck(binary, root)
}

func ValidateContractCheckWithDeps(binary, root string, deps contractauditworker.ValidationDeps) selfverify.StepResult {
	return contractauditworker.ValidateContractCheckWithDeps(binary, root, deps)
}

func ValidateToolConformance(binary, root string) selfverify.StepResult {
	return contractauditworker.ValidateToolConformance(binary, root)
}

func ValidateToolConformanceWithDeps(binary, root string, deps contractauditworker.ValidationDeps) selfverify.StepResult {
	return contractauditworker.ValidateToolConformanceWithDeps(binary, root, deps)
}

func ValidateWorkerLifecycle(binary, root string, seed int64) selfverify.StepResult {
	return contractauditworker.ValidateWorkerLifecycle(binary, root, seed)
}

func ValidateWorkerLifecycleWithDeps(binary, root string, seed int64, deps contractauditworker.ValidationDeps) selfverify.StepResult {
	return contractauditworker.ValidateWorkerLifecycleWithDeps(binary, root, seed, deps)
}
