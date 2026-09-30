package probe

import (
	selfverify "issueops/internal/contract/selfverify"

	"issueops/internal/adapter/verification/probe/goformat"
	"issueops/internal/adapter/verification/probe/nativeintegration"
)

func ValidateCommandPolicy(binary, root string) selfverify.StepResult {
	return validateCommandPolicy(binary, root)
}

func ValidateContractCheckSmoke(binary, root string) selfverify.StepResult {
	return ValidateContractCheck(binary, root)
}

func ValidateWorkerLifecycleSmoke(binary, root string, seed int64) selfverify.StepResult {
	return ValidateWorkerLifecycle(binary, root, seed)
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

func ValidateGoFormat(root string) selfverify.StepResult {
	return goformat.Validate(root)
}

func DetectClaudeMCPDuplicateWarnings(output string) []nativeintegration.ClaudeMCPDuplicateWarning {
	return nativeintegration.DetectClaudeMCPDuplicateWarnings(output)
}

func ClaudeMCPDuplicateWarningFixture() string {
	return nativeintegration.ClaudeMCPDuplicateWarningFixture()
}

func LintMermaidBlocks(relPath, text string) []string {
	return lintMermaidBlocks(relPath, text)
}

func FindUnredactedSecretLike(text string) []string {
	return findUnredactedSecretLike(text)
}

func ContainsForbiddenLegacyOutsideRuntimePaths(text, root string) bool {
	return containsForbiddenLegacyOutsideRuntimePaths(text, root)
}

func ForbiddenNameHits(root string) []string {
	return forbiddenNameHits(root)
}

func ValidateHarnessInvariants(root string) selfverify.StepResult {
	return validateHarnessInvariants(root)
}
