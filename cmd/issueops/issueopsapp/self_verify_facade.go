package issueopsapp

import (
	augmentcontract "issueops/internal/contract/selfaugment"
	"time"

	"issueops/cmd/issueops/selfworkflow"
)

func runSelfVerify(args []string) error {
	if len(args) > 0 && args[0] == "history" {
		return runSelfVerifyHistory(args[1:])
	}
	if len(args) > 0 && args[0] == "compare" {
		return runSelfVerifyCompare(args[1:])
	}
	if len(args) > 0 && args[0] == "promote" {
		return runSelfVerifyPromote(args[1:])
	}
	if len(args) > 0 && args[0] == "candidates" {
		return runSelfVerifyCandidates(args[1:])
	}
	return selfworkflow.RunSelfVerifyWithDeps(args, selfworkflow.SelfVerifyRunDeps{
		Verify: func(request selfworkflow.SelfVerifyRequest) (augmentcontract.SelfAugmentResult, error) {
			return selfVerify(request)
		},
	})
}

func runSelfVerifyCandidates(args []string) error {
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.RunSelfVerifyCandidates(args)
}

func runSelfVerifyCompare(args []string) error {
	return selfworkflow.RunSelfVerifyCompare(args)
}

func runSelfVerifyHistory(args []string) error {
	return selfworkflow.RunSelfVerifyHistory(args)
}

func runSelfVerifyPromote(args []string) error {
	return selfworkflow.RunSelfVerifyPromote(args)
}

func selfVerify(request selfworkflow.SelfVerifyRequest) (augmentcontract.SelfAugmentResult, error) {
	return selfworkflow.SelfVerify(request, selfVerifyLoopDeps())
}

func selfVerifyLoopDeps() selfworkflow.SelfVerifyLoopDeps {
	return selfworkflow.SelfVerifyLoopDeps{
		StepDeps:   selfVerifyStepDeps(),
		FailedStep: failedStep,
		PrintStep:  printStep,
	}
}

func selfVerifyStepDeps() selfworkflow.SelfVerifyStepDeps {
	return selfworkflow.SelfVerifyStepDeps{
		IssueOpsRoot:                    issueOpsRoot,
		RunCommandStep:                  runCommandStepAdapter,
		ValidateHarnessInvariants:       validateHarnessInvariants,
		ValidateGoFormat:                validateGoFormat,
		ValidateRiskQATier:              validateRiskQATierEvidence,
		ValidateInspect:                 validateInspect,
		ValidateDocsIndex:               validateDocsIndex,
		ValidateSelfVerifyCandidate:     validateSelfVerifyCandidateExport,
		ValidateStepBudgetBaseline:      validateStepBudgetBaseline,
		ValidateInstallDryRunSmoke:      validateInstallDryRunSmoke,
		ValidateCommandPolicy:           validateCommandPolicy,
		ValidateCommandAudit:            validateCommandAudit,
		ValidateContractCheck:           validateContractCheck,
		ValidateToolConformance:         validateToolConformance,
		ValidateWorkerLifecycle:         validateWorkerLifecycle,
		ValidateMCP:                     validateMCP,
		ValidateStateRoundtrip:          validateStateRoundtrip,
		ValidateParallelTempIsolation:   validateParallelTempIsolation,
		ValidateDaemonRestartResilience: validateDaemonRestartResilience,
		ValidatePreflightFuzz:           validatePreflightFuzz,
		ValidateWebFetchBattery:         validateWebFetchBattery,
		ValidateNativeIntegration:       validateNativeIntegration,
		ValidateRedactionAudit:          validateRedactionAudit,
		ValidateQAGate:                  validateQAGate,
	}
}

func runCommandStepAdapter(dir string, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return runCommandStep(dir, label, timeout, stdin, name, args...)
}
