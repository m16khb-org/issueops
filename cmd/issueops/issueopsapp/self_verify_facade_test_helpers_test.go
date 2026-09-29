package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	domain "issueops/internal/domain/selfaugment"
	"time"

	"issueops/cmd/issueops/selfworkflow"
)

func exportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.ExportSelfVerificationCandidates()
}

func selfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return selfworkflow.SelfVerificationCandidateCatalog()
}

func selfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return selfworkflow.SelfVerificationCandidateIDsByStatus(candidates, status)
}

func selectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return selfworkflow.SelectedSelfVerificationCandidateID(candidate)
}

func runSelfVerifyCandidatesWithDeps(args []string, deps selfVerifyCandidatesDeps) error {
	return selfworkflow.RunSelfVerifyCandidatesWithDeps(args, selfworkflow.SelfVerifyCandidatesDeps{
		Export: deps.export,
		Save:   deps.save,
	})
}

func saveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	return selfworkflow.SaveSelfVerificationCandidateExport(result, key)
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
	return selfworkflow.RunSelfVerifyPromoteWithDeps(args, selfworkflow.SelfVerifyPromoteDeps{Promote: deps.promote})
}

func promoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
	return selfworkflow.PromoteSelfAugmentBaseline(fromKey, baselineKey, confirm, allowFailedSource)
}

func readSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return selfworkflow.ReadSelfAugmentStateSnapshot(key)
}

func isSelfVerificationSummaryKind(kind string) bool {
	return selfworkflow.IsSelfVerificationSummaryKind(kind)
}

func writeSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return selfworkflow.WriteSelfAugmentSnapshotRecord(dir, key, snapshot)
}

func boolPtr(value bool) *bool {
	return &value
}

func newSelfVerifyLoopResult(iterations int, baseSeed int64, targetScore float64) SelfAugmentResult {
	selfworkflow.IssueOpsRoot = issueOpsRoot
	return selfworkflow.NewSelfVerifyLoopResult(iterations, baseSeed, targetScore)
}

func emitSelfVerifyLoopStart(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64) {
	if progress == nil {
		return
	}
	selfworkflow.EmitSelfVerifyLoopStart(progress.inner, loopKind, iterations, seed)
}

func emitSelfVerifyLoopEnd(progress *selfVerifyProgressReporter, loopKind string, iterations int, seed int64, ok bool, errorText string) {
	if progress == nil {
		return
	}
	selfworkflow.EmitSelfVerifyLoopEnd(progress.inner, loopKind, iterations, seed, ok, errorText)
}

func saveSelfVerificationSummary(result *SelfAugmentResult, key string) error {
	return selfworkflow.SaveSelfVerificationSummary(result, key)
}

func saveSelfAugmentSummary(result *SelfAugmentResult, key string) error {
	return selfworkflow.SaveSelfAugmentSummary(result, key)
}

func newSelfVerificationSummarySnapshot(result SelfAugmentResult, generatedAt time.Time) SelfAugmentStateSnapshot {
	return selfworkflow.NewSelfVerificationSummarySnapshot(result, generatedAt)
}

func plannedSelfVerifySteps(root string, tempBin string, seed int64, goTestStep *StepResult) []selfVerifyPlannedStep {
	return selfworkflow.PlannedSelfVerifySteps(root, tempBin, seed, goTestStep, selfVerifyStepDeps())
}

func cachedContractGoldenStep(goTestStep StepResult) StepResult {
	return selfworkflow.CachedContractGoldenStep(goTestStep, selfVerifyStepDeps())
}

type selfVerifyPlannedStep = selfworkflow.SelfVerifyPlannedStep

type selfVerifyCandidatesDeps struct {
	export func() SelfVerificationCandidateExportResult
	save   func(result *SelfVerificationCandidateExportResult, key string) error
}

type selfVerifyPromoteDeps struct {
	promote func(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error)
}

type SelfVerificationCandidateExportResult = selfworkflow.SelfVerificationCandidateExportResult

type SelfVerificationCandidate = selfworkflow.SelfVerificationCandidate
