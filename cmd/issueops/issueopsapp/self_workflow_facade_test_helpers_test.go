package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	augmentcontract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
	"time"

	"issueops/cmd/issueops/selfworkflow"
)

func applySelfAugmentHistoryRetention(result *SelfAugmentHistoryResult, options selfAugmentHistoryRetentionOptions) error {
	return newSelfWorkflowHistory(statestore.StateDir()).ApplyRetention(result, options)
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
	return selfworkflow.CollectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return selfworkflow.SelfAugmentCandidates(signals)
}

func scoreBool(ok bool) float64 {
	return selfworkflow.ScoreBool(ok)
}

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return selfworkflow.AllSelfAugmentGoalsPassed(goals)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return selfworkflow.SelectedCandidateID(candidate)
}

func docsContainTerm(root, term string) bool {
	return selfworkflow.DocsContainTerm(root, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return selfworkflow.FileContainsTerm(root, relPath, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return selfworkflow.DirContainsTerm(root, relDir, term)
}

func selectGeniusFormulas(text string) []string {
	return selfworkflow.SelectGeniusFormulas(text)
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return selfworkflow.SelfAugmentResearchInfluences()
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	selfworkflow.MarkSatisfiedSelfAugmentCandidate(candidate, signals)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return selfworkflow.SelfAugmentCandidateScore(candidate)
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
	return selfworkflow.StepDurationStatByLabel(stats)
}

func maxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return selfworkflow.MaxSlowStepDurationByLabel(steps)
}

func buildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return selfworkflow.BuildStepDurationStats(durationsByLabel)
}

func stepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return selfworkflow.StepDurationStatsForCompare(summary)
}

func summarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return selfworkflow.SummarizeSelfAugment(result)
}

func summarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return selfworkflow.SummarizeSelfVerification(result, targetScore)
}

func classifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return selfworkflow.ClassifySelfVerificationFailure(result, summary)
}

func selfVerificationFailureClusters(result SelfAugmentResult) []SelfVerificationFailureCluster {
	return selfworkflow.SelfVerificationFailureClusters(result)
}

func selfVerifyRerunCommands(failedStep string, baseSeed int64, targetScore float64) []string {
	return selfworkflow.SelfVerifyRerunCommands(failedStep, baseSeed, targetScore)
}

func selfVerifyStepRerunCommand(label string) (string, bool) {
	return selfworkflow.SelfVerifyStepRerunCommand(label)
}

func formatScore(score float64) string {
	return selfworkflow.FormatScore(score)
}

func scoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return selfworkflow.ScoreSelfVerificationGoals(result, targetScore)
}

func selfVerificationContract() SelfVerificationContract {
	return selfworkflow.BuildSelfVerificationContract()
}

func selfVerificationCoverage(stepLabels []string) ([]SelfVerificationCoverage, []string) {
	return selfworkflow.BuildSelfVerificationCoverage(stepLabels)
}

func selfVerificationCoverageDefinitions() []selfVerificationCoverageDefinition {
	return selfworkflow.SelfVerificationCoverageDefinitions()
}

type selfAugmentHistoryRetentionOptions = augmentcontract.SelfAugmentHistoryRetentionOptions

type selfVerificationCoverageDefinition = selfworkflow.SelfVerificationCoverageDefinition

type SelfAugmentPromoteResult = augmentcontract.SelfAugmentPromoteResult

type SelfAugmentIteration = selfworkflow.SelfAugmentIteration

type SelfAugmentCompareResult = augmentcontract.SelfAugmentCompareResult

type SelfAugmentSlowStepRegression = augmentcontract.SelfAugmentSlowStepRegression

type SelfAugmentStepBudgetRegression = augmentcontract.SelfAugmentStepBudgetRegression

type SelfAugmentHistoryResult = augmentcontract.SelfAugmentHistoryResult

type SelfAugmentHistoryEntry = augmentcontract.SelfAugmentHistoryEntry

type SelfAugmentInfluence = selfworkflow.SelfAugmentInfluence

type SelfAugmentGoal = selfworkflow.SelfAugmentGoal

type SelfAugmentCandidate = selfworkflow.SelfAugmentCandidate

type SelfAugmentRepoSignals = selfworkflow.SelfAugmentRepoSignals

type SelfAugmentStateSnapshot = augmentcontract.SelfAugmentStateSnapshot

type SelfAugmentSummary = selfworkflow.SelfAugmentSummary

type SelfVerifyLLMEvalResult = selfworkflow.SelfVerifyLLMEvalResult

type SelfVerificationContract = selfworkflow.SelfVerificationContract

type SelfVerificationGoalScore = selfworkflow.SelfVerificationGoalScore

type SelfVerificationCoverage = selfworkflow.SelfVerificationCoverage

type SelfVerificationFailureCluster = selfworkflow.SelfVerificationFailureCluster

type SelfAugmentSlowStep = selfworkflow.SelfAugmentSlowStep

type SelfAugmentStepDurationStat = selfworkflow.SelfAugmentStepDurationStat

const selfVerificationSummaryKind = selfworkflow.SelfVerificationSummaryKind

type SelfAugmentPlanRequest = selfworkflow.SelfAugmentPlanRequest
type SelfAugmentPlanResult = selfworkflow.SelfAugmentPlanResult

type SelfAugmentResult = selfworkflow.SelfAugmentResult
