package issueopsapp

import (
	"context"
	docsapp "issueops/internal/application/docs"
	"time"

	augmentation "issueops/internal/adapter/augmentation"
	docs "issueops/internal/adapter/docs"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
)

func applySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options selfAugmentHistoryRetentionOptions) error {
	return newSelfWorkflowHistory(statestore.StateDir()).ApplyRetention(context.Background(), result, options)
}

func parseSelfAugmentTimestamp(value string) (time.Time, bool) {
	return domain.ParseHistoryTimestamp(value)
}

func nonNilStringSlice(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}

func nonNilSlowStepSlice(items []SelfAugmentSlowStep) []SelfAugmentSlowStep {
	if items == nil {
		return []SelfAugmentSlowStep{}
	}
	return items
}

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).CollectSignals(root, docsIndexed, skills, geniusText)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return augmentapp.Candidates(signals)
}

func scoreBool(ok bool) float64 {
	return domain.ScoreBool(ok)
}

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return domain.AllGoalsPassed(goals)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return domain.SelectedCandidateID(candidate)
}

func docsContainTerm(root, term string) bool {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).DocsContainTerm(root, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return augmentation.FileContainsTerm(root, relPath, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return augmentation.DirContainsTerm(root, relDir, term)
}

func selectGeniusFormulas(text string) []string {
	return domain.SelectGeniusFormulas(text)
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return domain.ResearchInfluences()
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	domain.MarkSatisfiedCandidate(candidate, signals)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return domain.CandidateScore(candidate)
}

func compareSlowestStepRegressions(baseline, candidate []SelfAugmentSlowStep, maxRegressionPct float64) []SelfAugmentSlowStepRegression {
	return domain.CompareSlowestStepRegressions(baseline, candidate, maxRegressionPct)
}

func compareStepBudgetRegressions(baseline, candidate []SelfAugmentStepDurationStat, maxRegressionPct float64) []SelfAugmentStepBudgetRegression {
	return domain.CompareStepBudgetRegressions(baseline, candidate, maxRegressionPct)
}

func missingStrings(want, have []string) []string {
	return domain.MissingStrings(want, have)
}

func stepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return domain.StepDurationStatByLabel(stats)
}

func maxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return domain.MaxSlowStepDurationByLabel(steps)
}

func buildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return domain.BuildStepDurationStats(durationsByLabel)
}

func stepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return domain.StepDurationStatsForCompare(summary)
}

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return verifyapp.SummarizeSelfVerification(result, 95)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return verifyapp.SummarizeSelfVerification(result, targetScore)
}

func classifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return verifyapp.ClassifySelfVerificationFailure(result, summary)
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return verifyapp.SelfVerificationFailureClusters(result)
}

func selfVerifyRerunCommands(failedStep string, baseSeed int64, targetScore float64) []string {
	return verifydomain.SelfVerifyRerunCommands(failedStep, baseSeed, targetScore)
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	return verifydomain.SelfVerifyStepRerunCommand(label)
}

func formatScore(score float64) string {
	return verifydomain.FormatScore(score)
}

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return verifyapp.MapGoalScores(result, targetScore)
}

func selfVerificationContract() SelfVerificationContract {
	return verifydomain.ContractValue()
}

func selfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return verifydomain.CoverageForLabels(stepLabels)
}

func selfVerificationCoverageDefinitions() []selfVerificationCoverageDefinition {
	return verifydomain.CoverageDefinitions()
}

type selfAugmentHistoryRetentionOptions = augmentcontract.SelfAugmentHistoryRetentionOptions

type selfVerificationCoverageDefinition = verifycontract.SelfVerificationCoverageDefinition

type SelfAugmentPromoteResult = augmentcontract.SelfAugmentPromoteResult

type SelfAugmentIteration = augmentcontract.SelfAugmentIteration

type SelfAugmentCompareResult = augmentcontract.SelfAugmentCompareResult

type SelfAugmentSlowStepRegression = augmentcontract.SelfAugmentSlowStepRegression

type SelfAugmentStepBudgetRegression = augmentcontract.SelfAugmentStepBudgetRegression

type SelfAugmentHistoryResult = augmentcontract.SelfAugmentHistoryResult

type SelfAugmentHistoryEntry = augmentcontract.SelfAugmentHistoryEntry

type SelfAugmentInfluence = augmentcontract.SelfAugmentInfluence

type SelfAugmentGoal = augmentcontract.SelfAugmentGoal

type SelfAugmentCandidate = augmentcontract.SelfAugmentCandidate

type SelfAugmentRepoSignals = augmentcontract.SelfAugmentRepoSignals

type SelfAugmentStateSnapshot = augmentcontract.SelfAugmentStateSnapshot

type SelfAugmentSummary = augmentcontract.SelfAugmentSummary

type SelfVerifyLLMEvalResult = augmentcontract.SelfVerifyLLMEvalResult

type SelfVerificationContract = verifycontract.SelfVerificationContract

type SelfVerificationGoalScore = verifycontract.SelfVerificationGoalScore

type SelfVerificationCoverage = verifycontract.SelfVerificationCoverage

type SelfVerificationFailureCluster = verifycontract.SelfVerificationFailureCluster

type SelfAugmentSlowStep = augmentcontract.SelfAugmentSlowStep

type SelfAugmentStepDurationStat = augmentcontract.SelfAugmentStepDurationStat

const selfVerificationSummaryKind = domain.SelfVerificationSummaryKind

type SelfAugmentPlanRequest = augmentcontract.SelfAugmentPlanRequest
type SelfAugmentPlanResult = augmentcontract.SelfAugmentPlanResult

type SelfAugmentResult = augmentcontract.SelfAugmentResult
