package issueopsapp

import (
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
	verifydomain "issueops/internal/domain/selfverify"
)

func exportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).ExportCandidates()
}

func selfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return verifydomain.CandidateCatalog()
}

func selfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return verifydomain.CandidateIDsByStatus(candidates, status)
}

func selectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return verifydomain.SelectedCandidateID(candidate)
}

func runSelfVerifyCandidatesWithDeps(args []string, deps selfVerifyCandidatesDeps) error {
	return candidatescmd.Run(args, candidatescmd.Deps{PrintJSON: printJSON,
		Export: deps.export,
		Save:   deps.save,
	})
}

func saveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	return newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version).SaveCandidates(result, key)
}

func compareSelfAugmentSummaries(baselineKey, candidateKey string, maxElapsedRegressionPct float64) (SelfAugmentCompareResult, error) {
	return newSelfWorkflowHistory(statestore.StateDir()).Compare(baselineKey, candidateKey, maxElapsedRegressionPct)
}

func compareSelfAugmentSummariesFromSnapshots(baselineKey, candidateKey string, maxElapsedRegressionPct float64, baseline, candidate SelfAugmentStateSnapshot) SelfAugmentCompareResult {
	return app.CompareSnapshots(baselineKey, candidateKey, maxElapsedRegressionPct, baseline, candidate, statestore.StateDir())
}

func newSelfAugmentCompareResult(baselineKey, candidateKey string, maxElapsedRegressionPct float64) SelfAugmentCompareResult {
	return domain.NewCompareResult(baselineKey, candidateKey, maxElapsedRegressionPct, statestore.StateDir())
}

func selfAugmentHistory(prefix string, limit int, retentionOptions ...selfAugmentHistoryRetentionOptions) (SelfAugmentHistoryResult, error) {
	options := selfAugmentHistoryRetentionOptions{}
	if len(retentionOptions) > 0 {
		options = retentionOptions[0]
	}
	return newSelfWorkflowHistory(statestore.StateDir()).History(prefix, limit, options)
}

func runSelfVerifyPromoteWithDeps(args []string, deps selfVerifyPromoteDeps) error {
	return promotecmd.Run(args, promotecmd.Deps{Promote: deps.promote, PrintJSON: printJSON})
}

func promoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
	return newSelfWorkflowState(statestore.StateDir()).Promote(fromKey, baselineKey, confirm, allowFailedSource)
}

func readSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return (app.SnapshotStore{ReadState: newSelfWorkflowStateService(statestore.StateDir()).Read}).Read(key)
}

func isSelfVerificationSummaryKind(kind string) bool {
	return domain.IsSelfVerificationSummaryKind(kind)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return (app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: newSelfWorkflowStateService(dir).WriteRecord, Now: time.Now}).Write(dir, key, snapshot)
}

func boolPtr(value bool) *bool {
	return &value
}

func newSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) SelfAugmentResult {
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

func saveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	return newSelfWorkflowState(statestore.StateDir()).SaveSummary(result, key)
}

func saveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return newSelfWorkflowState(statestore.StateDir()).SaveSummary(result, key)
}

func newSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return domain.NewSelfVerificationSummarySnapshot(result, generatedAt)
}

func plannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult) []selfVerifyPlannedStep {
	return verifyapp.PlannedSteps(root, tempBin, seed, goTestStep, selfVerifyStepDeps(issueOpsRoot()))
}

func cachedContractGoldenStep(goTestStep StepResult) StepResult {
	return verifyapp.CachedContractGoldenStep(goTestStep, selfVerifyStepDeps(issueOpsRoot()))
}

type selfVerifyPlannedStep = verifyapp.SelfVerifyPlannedStep

type selfVerifyCandidatesDeps struct {
	export func() SelfVerificationCandidateExportResult
	save   func(result *SelfVerificationCandidateExportResult, key string) error
}

type selfVerifyPromoteDeps struct {
	promote func(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error)
}

type SelfVerificationCandidateExportResult = augmentcontract.SelfVerificationCandidateExportResult

type SelfVerificationCandidate = verifycontract.SelfVerificationCandidate

func saveSelfAugmentPlan(result *augmentcontract.SelfAugmentPlanResult, key string) error {
	return newSelfWorkflowState(statestore.StateDir()).SavePlan(result, key)
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
