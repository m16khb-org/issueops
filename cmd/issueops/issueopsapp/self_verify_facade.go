package issueopsapp

import (
	"context"
	commandstep "issueops/cmd/issueops/commandstep"
	candidateexport "issueops/internal/adapter/verification/probe/candidateexport"
	commandpolicy "issueops/internal/adapter/verification/probe/commandpolicy"
	contractauditworker "issueops/internal/adapter/verification/probe/contractauditworker"
	installdryrun "issueops/internal/adapter/verification/probe/installdryrun"
	invariants "issueops/internal/adapter/verification/probe/invariants"
	mcpsmoke "issueops/internal/adapter/verification/probe/mcpsmoke"
	parallelisolation "issueops/internal/adapter/verification/probe/parallelisolation"
	smoke "issueops/internal/adapter/verification/probe/smoke"
	selfverify "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"

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
		SaveSummary: func(result *augmentcontract.SelfAugmentResult, key string) error {
			return newSelfWorkflowState(statestore.StateDir()).SaveSummary(context.Background(), result, key)
		},
		PrintJSON: printJSON,
		Verify:    newSelfWorkflowExecutor(issueOpsRoot()),
	})
}

func runSelfVerifyCandidates(args []string) error {
	planning := newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version)
	return candidatescmd.Run(args, candidatescmd.Deps{Export: planning.ExportCandidates, Save: func(result *augmentcontract.SelfVerificationCandidateExportResult, key string) error {
		return planning.SaveCandidates(context.Background(), result, key)
	}, PrintJSON: printJSON})
}

func runSelfVerifyCompare(args []string) error {
	return historycompare.RunSelfVerifyCompare(args, selfWorkflowHistoryCLI())
}

func runSelfVerifyHistory(args []string) error {
	return historycompare.RunSelfVerifyHistory(args, selfWorkflowHistoryCLI())
}

func runSelfVerifyPromote(args []string) error {
	return promotecmd.Run(args, promotecmd.Deps{Promote: func(from, to string, confirm, allowFailed bool) (augmentcontract.SelfAugmentPromoteResult, error) {
		return newSelfWorkflowState(statestore.StateDir()).Promote(context.Background(), from, to, confirm, allowFailed)
	}, PrintJSON: printJSON})
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
		FailedStep:     selfverifydomain.FailedStep,
		PrintStep:      commandstep.PrintStep,
	}
}

func selfVerifyStepDeps(root string) app.SelfVerifyStepDeps {
	stateProbe := newStateRoundtripProbe()
	budgetProbe := newStepBudgetProbe()
	docsProbe := newDocsQAProbe()
	return app.SelfVerifyStepDeps{
		IssueOpsRoot:                func() string { return root },
		RunCommandStep:              runCommandStep,
		ValidateHarnessInvariants:   invariants.ValidateHarnessInvariants,
		ValidateGoFormat:            validateGoFormat,
		ValidateRiskQATier:          validateRiskQATierEvidence,
		ValidateRiskQATierWithScope: validateRiskQATierEvidenceWithScope,
		ValidateInspect:             smoke.ValidateInspect,
		ValidateDocsIndex:           smoke.ValidateDocsIndex,
		ValidateSelfVerifyCandidate: candidateexport.ValidateSelfVerifyCandidateExport,
		ValidateStepBudgetBaseline: func(binary, root string, seed int64) selfverify.StepResult {
			return stepbudget.ValidateStepBudgetBaselineWithDeps(binary, root, seed, budgetProbe)
		},
		ValidateInstallDryRunSmoke:    installdryrun.Validate,
		ValidateCommandPolicy:         commandpolicy.Validate,
		ValidateCommandAudit:          contractauditworker.ValidateCommandAudit,
		ValidateContractCheck:         contractauditworker.ValidateContractCheck,
		ValidateToolConformance:       contractauditworker.ValidateToolConformance,
		ValidateWorkerLifecycle:       contractauditworker.ValidateWorkerLifecycle,
		ValidateMCP:                   mcpsmoke.ValidateMCP,
		ValidateStateRoundtrip:        stateProbe.Validate,
		ValidateParallelTempIsolation: parallelisolation.Validate,
		ValidatePreflightFuzz:         (preflightfuzz.Validator{Git: preflightadapter.GitCmd}).Validate,
		ValidateWebFetchBattery:       newWebFetchProbe().Validate,
		ValidateNativeIntegration:     newNativeIntegrationProbe().Validate,
		ValidateRedactionAudit:        docsProbe.RedactionAudit,
		ValidateQAGate:                docsProbe.Validate,
	}
}
