package issueopsapp

import (
	"fmt"
	preflightadapter "issueops/internal/adapter/preflight"
	"issueops/internal/adapter/verification/probe/preflightfuzz"
	"issueops/internal/adapter/verification/probe/stepbudget"
	"os"
	"path/filepath"
	"time"

	"issueops/cmd/issueops/selfworkflow/candidatescmd"
	"issueops/cmd/issueops/selfworkflow/historycompare"
	"issueops/cmd/issueops/selfworkflow/promotecmd"
	"issueops/cmd/issueops/selfworkflow/verifycmd"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
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
	return verifycmd.Run(args, verifycmd.Deps{
		SaveSummary: newSelfWorkflowState(statestore.StateDir()).SaveSummary,
		PrintJSON:   printJSON,
		Verify:      newSelfWorkflowExecutor(issueOpsRoot()),
	})
}

func runSelfVerifyCandidates(args []string) error {
	planning := newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version)
	return candidatescmd.Run(args, candidatescmd.Deps{Export: planning.ExportCandidates, Save: planning.SaveCandidates, PrintJSON: printJSON})
}

func runSelfVerifyCompare(args []string) error {
	return historycompare.RunSelfVerifyCompare(args, selfWorkflowHistoryCLI())
}

func runSelfVerifyHistory(args []string) error {
	return historycompare.RunSelfVerifyHistory(args, selfWorkflowHistoryCLI())
}

func runSelfVerifyPromote(args []string) error {
	return promotecmd.Run(args, promotecmd.Deps{Promote: newSelfWorkflowState(statestore.StateDir()).Promote, PrintJSON: printJSON})
}

func newSelfWorkflowExecutor(root string) func(app.LoopRequest) (augmentcontract.SelfAugmentResult, error) {
	deps := selfVerifyLoopDeps(root)
	return func(request app.LoopRequest) (augmentcontract.SelfAugmentResult, error) {
		return app.ExecuteLoop(request, deps)
	}
}

func selfVerifyLoopDeps(root string) app.LoopDeps {
	return app.LoopDeps{
		IssueOpsRoot:   func() string { return root },
		StepDeps:       selfVerifyStepDeps(root),
		Printf:         fmt.Printf,
		Summarize:      app.SummarizeSelfVerification,
		MkdirTemp:      func() (string, error) { return os.MkdirTemp("", "issueops-self-verify-*") },
		RemoveAll:      os.RemoveAll,
		TempBinaryPath: func(dir string) string { return filepath.Join(dir, "issueops") },
		Now:            time.Now,
		FailedStep:     failedStep,
		PrintStep:      printStep,
	}
}

func selfVerifyStepDeps(root string) app.SelfVerifyStepDeps {
	stateProbe := newStateRoundtripProbe()
	budgetProbe := newStepBudgetProbe()
	docsProbe := newDocsQAProbe()
	return app.SelfVerifyStepDeps{
		IssueOpsRoot:                func() string { return root },
		RunCommandStep:              runCommandStepAdapter,
		ValidateHarnessInvariants:   validateHarnessInvariants,
		ValidateGoFormat:            validateGoFormat,
		ValidateRiskQATier:          validateRiskQATierEvidence,
		ValidateInspect:             validateInspect,
		ValidateDocsIndex:           validateDocsIndex,
		ValidateSelfVerifyCandidate: validateSelfVerifyCandidateExport,
		ValidateStepBudgetBaseline: func(binary, root string, seed int64) StepResult {
			return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, budgetProbe)
		},
		ValidateInstallDryRunSmoke:      validateInstallDryRunSmoke,
		ValidateCommandPolicy:           validateCommandPolicy,
		ValidateCommandAudit:            validateCommandAudit,
		ValidateContractCheck:           validateContractCheck,
		ValidateToolConformance:         validateToolConformance,
		ValidateWorkerLifecycle:         validateWorkerLifecycle,
		ValidateMCP:                     validateMCP,
		ValidateStateRoundtrip:          stateProbe.Validate,
		ValidateParallelTempIsolation:   validateParallelTempIsolation,
		ValidateDaemonRestartResilience: validateDaemonRestartResilience,
		ValidatePreflightFuzz:           (preflightfuzz.Validator{Git: preflightadapter.GitCmd}).Validate,
		ValidateWebFetchBattery:         newWebFetchProbe().Validate,
		ValidateNativeIntegration:       newNativeIntegrationProbe().Validate,
		ValidateRedactionAudit:          docsProbe.RedactionAudit,
		ValidateQAGate:                  docsProbe.Validate,
	}
}

func runCommandStepAdapter(dir string, label string, timeout time.Duration, stdin string, name string, args ...string) StepResult {
	return runCommandStep(dir, label, timeout, stdin, name, args...)
}
