package issueopsapp

import (
	"time"

	"issueops/internal/adapter/verification"
	probe "issueops/internal/adapter/verification/probe"
	selfverifyapp "issueops/internal/application/selfverify"
)

func validateInspect(binary, root string) StepResult {
	return probe.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) StepResult {
	return probe.ValidateDocsIndex(binary, root)
}

func validateCommandPolicy(binary, root string) StepResult {
	return probe.ValidateCommandPolicy(binary, root)
}

func validateMCP(binary, root string) StepResult {
	return probe.ValidateMCP(binary, root)
}

func validateInstallDryRunSmoke(binary, root string, seed int64) StepResult {
	return probe.ValidateInstallDryRunSmoke(binary, root, seed)
}

func validateParallelTempIsolation(binary, root string, seed int64) StepResult {
	return probe.ValidateParallelTempIsolation(binary, root, seed)
}

func validateDaemonRestartResilience(binary, root string, seed int64) StepResult {
	return probe.ValidateDaemonRestartResilience(binary, root, seed)
}

func validatePreflightFuzz(binary, root string, seed int64) StepResult {
	return probe.ValidatePreflightFuzz(binary, root, seed)
}

func validateWebFetchBattery(binary, root string, seed int64) StepResult {
	return probe.ValidateWebFetchBattery(binary, root, seed)
}

func validateCommandAudit(binary, root string, seed int64) StepResult {
	return probe.ValidateCommandAudit(binary, root, seed)
}

func validateContractCheck(binary, root string) StepResult {
	return probe.ValidateContractCheck(binary, root)
}

func validateToolConformance(binary, root string) StepResult {
	return probe.ValidateToolConformance(binary, root)
}

func validateWorkerLifecycle(binary, root string, seed int64) StepResult {
	return probe.ValidateWorkerLifecycle(binary, root, seed)
}

func validateSelfVerifyCandidateExport(binary, root string, seed int64) StepResult {
	return probe.ValidateSelfVerifyCandidateExport(binary, root, seed)
}

func validateGoFormat(root string) StepResult {
	return selfverifyapp.ValidateFormat(root, selfverifyapp.FormatDeps{
		ListTrackedGoFiles: verification.ListTrackedGoFiles,
		ListUnformatted:    verification.ListUnformatted,
		Now:                time.Now,
	})
}

func validateHarnessInvariants(root string) StepResult {
	return probe.ValidateHarnessInvariants(root)
}

func validateNativeIntegration(root string) StepResult {
	return probe.ValidateNativeIntegration(root)
}
