package issueopsapp

import (
	"context"
	statecontract "issueops/internal/contract/state"
	statepath "issueops/internal/domain/statepath"
	"os"
	"time"

	"issueops/cmd/issueops/selfworkflow/candidatescmd"
	"issueops/cmd/issueops/selfworkflow/promotecmd"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfaugment"
)

func exportSelfVerificationCandidates() augmentcontract.SelfVerificationCandidateExportResult {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).ExportCandidates()
}

func runSelfVerifyCandidatesWithDeps(args []string, deps selfVerifyCandidatesDeps) error {
	return candidatescmd.Run(args, candidatescmd.Deps{PrintJSON: printJSON,
		Export: deps.export,
		Save:   deps.save,
	})
}

func saveSelfVerificationCandidateExport(result *augmentcontract.SelfVerificationCandidateExportResult, key string) error {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).SaveCandidates(context.Background(), result, key)
}

func compareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (augmentcontract.SelfAugmentCompareResult, error) {
	return newSelfWorkflowHistory(statestore.StateDir()).Compare(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func compareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate augmentcontract.SelfAugmentStateSnapshot) augmentcontract.SelfAugmentCompareResult {
	return app.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, statestore.StateDir())
}

func newSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) augmentcontract.SelfAugmentCompareResult {
	return domain.NewCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct, statestore.StateDir())
}

func selfAugmentHistory(prefix string, limit int, retentionOptions ...augmentcontract.SelfAugmentHistoryRetentionOptions) (augmentcontract.SelfAugmentHistoryResult, error) {
	options := augmentcontract.SelfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		options = retentionOptions[0]
	}
	return newSelfWorkflowHistory(statestore.StateDir()).History(context.Background(), prefix, limit, options)
}

func runSelfVerifyPromoteWithDeps(args []string, deps selfVerifyPromoteDeps) error {
	return promotecmd.Run(args, promotecmd.Deps{Promote: deps.promote, PrintJSON: printJSON})
}

func promoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (augmentcontract.SelfAugmentPromoteResult, error) {
	return newSelfWorkflowState(statestore.StateDir()).Promote(context.Background(), fromKey, baselineKey, confirm, allowFailedSource)
}

func readSelfAugmentStateSnapshot(key string) (augmentcontract.SelfAugmentStateSnapshot, error) {
	return (app.SnapshotStore{ReadState: newStateService(statestore.StateDir()).Read}).Read(key)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot augmentcontract.SelfAugmentStateSnapshot) error {
	return (app.SnapshotStore{NormalizeKey: statepath.NormalizeKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
		return newStateService(dir).WriteRecord(context.Background(), dir, key, record)
	}, Now: time.Now}).Write(dir, key, snapshot)
}

func boolPtr(value bool) *bool {
	return &value
}

func newSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) augmentcontract.SelfAugmentResult {
	return verifyapp.NewLoopResult(iterations, baseSeed, targetScore, selfWorkflowRootForTest())
}

func emitSelfVerifyLoopStart(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if progress == nil {
		return
	}
	progress.inner.Emit(verifycontract.ProgressEvent{Event: "loop_start", LoopKind: loopKind, Iterations: iterations, Seed: seed})
}

func emitSelfVerifyLoopEnd(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if progress == nil {
		return
	}
	progress.inner.Emit(verifycontract.ProgressEvent{Event: "loop_end", LoopKind: loopKind, Iterations: iterations, Seed: seed, OK: boolPtr(ok), Error: errorText})
}

func saveSelfVerificationSummary(result *augmentcontract.SelfAugmentResult, key string) error {
	return newSelfWorkflowState(statestore.StateDir()).SaveSummary(context.Background(), result, key)
}

func saveSelfAugmentSummary(result *augmentcontract.SelfAugmentResult, key string) error {
	return newSelfWorkflowState(statestore.StateDir()).SaveSummary(context.Background(), result, key)
}

func plannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *verifycontract.StepResult) []verifyapp.SelfVerifyPlannedStep {
	return verifyapp.PlannedSteps(root, tempBin, seed, goTestStep, selfVerifyStepDeps(issueOpsRoot()))
}

func cachedContractGoldenStep(goTestStep verifycontract.StepResult) verifycontract.StepResult {
	return verifyapp.CachedContractGoldenStep(goTestStep, selfVerifyStepDeps(issueOpsRoot()))
}

type selfVerifyCandidatesDeps struct {
	export func() augmentcontract.SelfVerificationCandidateExportResult
	save   func(result *augmentcontract.SelfVerificationCandidateExportResult, key string) error
}

type selfVerifyPromoteDeps struct {
	promote func(fromKey, baselineKey string, confirm, allowFailedSource bool) (augmentcontract.SelfAugmentPromoteResult, error)
}

func selfWorkflowRootForTest() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
