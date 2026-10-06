package probe

import (
	selfverify "issueops/internal/contract/selfverify"
)

func ValidateCommandPolicy(binary, root string) selfverify.StepResult {
	return validateCommandPolicy(binary, root)
}

func ValidateParallelTempIsolation(binary, root string, seed int64) selfverify.StepResult {
	return validateParallelTempIsolation(binary, root, seed)
}

func ValidateInstallDryRunSmoke(binary, root string, seed int64) selfverify.StepResult {
	return validateInstallDryRunSmoke(binary, root, seed)
}

func ValidateDaemonRestartResilience(binary, root string, seed int64) selfverify.StepResult {
	return validateDaemonRestartResilience(binary, root, seed)
}

func ValidateInspect(binary, root string) selfverify.StepResult {
	return validateInspect(binary, root)
}

func ValidateDocsIndex(binary, root string) selfverify.StepResult {
	return validateDocsIndex(binary, root)
}

func ValidateHarnessInvariants(root string) selfverify.StepResult {
	return validateHarnessInvariants(root)
}
