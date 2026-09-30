package issueopsapp

import (
	selfverify "issueops/internal/contract/selfverify"

	"time"

	"issueops/internal/adapter/verification"
	probe "issueops/internal/adapter/verification/probe"
	selfverifyapp "issueops/internal/application/selfverify"
)

func validateInspect(binary, root string) selfverify.StepResult {
	return probe.ValidateInspect(binary, root)
}

func validateDocsIndex(binary, root string) selfverify.StepResult {
	return probe.ValidateDocsIndex(binary, root)
}

func validateCommandPolicy(binary, root string) selfverify.StepResult {
	return probe.ValidateCommandPolicy(binary, root)
}

func validateMCP(binary, root string) selfverify.StepResult {
	return probe.ValidateMCP(binary, root)
}

func validateInstallDryRunSmoke(binary, root string, seed int64) selfverify.StepResult {
	return probe.ValidateInstallDryRunSmoke(binary, root, seed)
}

func validateParallelTempIsolation(binary, root string, seed int64) selfverify.StepResult {
	return probe.ValidateParallelTempIsolation(binary, root, seed)
}

func validateDaemonRestartResilience(binary, root string, seed int64) selfverify.StepResult {
	return probe.ValidateDaemonRestartResilience(binary, root, seed)
}

func validateCommandAudit(binary, root string, seed int64) selfverify.StepResult {
	return probe.ValidateCommandAudit(binary, root, seed)
}

func validateContractCheck(binary, root string) selfverify.StepResult {
	return probe.ValidateContractCheck(binary, root)
}

func validateToolConformance(binary, root string) selfverify.StepResult {
	return probe.ValidateToolConformance(binary, root)
}

func validateWorkerLifecycle(binary, root string, seed int64) selfverify.StepResult {
	return probe.ValidateWorkerLifecycle(binary, root, seed)
}

func validateSelfVerifyCandidateExport(binary, root string, seed int64) selfverify.StepResult {
	return probe.ValidateSelfVerifyCandidateExport(binary, root, seed)
}

func validateGoFormat(root string) selfverify.StepResult {
	return selfverifyapp.ValidateFormat(root, selfverifyapp.FormatDeps{
		ListTrackedGoFiles: verification.ListTrackedGoFiles,
		ListUnformatted:    verification.ListUnformatted,
		Now:                time.Now,
	})
}

func validateHarnessInvariants(root string) selfverify.StepResult {
	return probe.ValidateHarnessInvariants(root)
}
