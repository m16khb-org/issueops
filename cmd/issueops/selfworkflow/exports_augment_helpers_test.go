package selfworkflow

import (
	augmentation "issueops/internal/adapter/augmentation"
	docs "issueops/internal/adapter/docs"
	docsapp "issueops/internal/application/docs"
	augmentapp "issueops/internal/application/selfaugment"
	domain "issueops/internal/domain/selfaugment"
	"time"
)

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return domain.AllGoalsPassed(goals)
}

func AllSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return allSelfAugmentGoalsPassed(goals)
}

func BuildStepDurationStats(durationsByLabel map[string][]int64) []SelfAugmentStepDurationStat {
	return buildStepDurationStats(durationsByLabel)
}

func ClassifySelfVerificationFailure(result SelfAugmentResult, summary SelfAugmentSummary) (string, string, []SelfVerificationFailureCluster) {
	return classifySelfVerificationFailure(result, summary)
}

func CollectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return collectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
}

func collectSelfAugmentRepoSignals(root string, docsIndexed int, skills []string, geniusText string) SelfAugmentRepoSignals {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).CollectSignals(root, docsIndexed, skills, geniusText)
}

func DocsContainTerm(root, term string) bool {
	return docsContainTerm(root, term)
}

func docsContainTerm(root, term string) bool {
	return (augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}).DocsContainTerm(root, term)
}

func DirContainsTerm(root, relDir, term string) bool {
	return dirContainsTerm(root, relDir, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return augmentation.DirContainsTerm(root, relDir, term)
}

func FileContainsTerm(root, relPath, term string) bool {
	return fileContainsTerm(root, relPath, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return augmentation.FileContainsTerm(root, relPath, term)
}

func FormatScore(score float64) string {
	return formatScore(score)
}

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	markSatisfiedSelfAugmentCandidate(candidate, signals)
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	domain.MarkSatisfiedCandidate(candidate, signals)
}

func MaxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return maxSlowStepDurationByLabel(steps)
}

func ScoreBool(ok bool) float64 {
	return scoreBool(ok)
}

func scoreBool(ok bool) float64 {
	return domain.ScoreBool(ok)
}

func ScoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return scoreSelfVerificationGoals(result, targetScore)
}

func SelectGeniusFormulas(text string) []string {
	return selectGeniusFormulas(text)
}

func selectGeniusFormulas(text string) []string {
	return domain.SelectGeniusFormulas(text)
}

func SelectedCandidateID(candidate *SelfAugmentCandidate) string {
	return selectedCandidateID(candidate)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return domain.SelectedCandidateID(candidate)
}

func SelfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return selfAugmentCandidates(signals)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return augmentapp.Candidates(signals)
}

func SelfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return selfAugmentCandidateScore(candidate)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return domain.CandidateScore(candidate)
}

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return domain.CandidateIDsByStatus(candidates, status)
}

func SelfAugmentResearchInfluences() []SelfAugmentInfluence {
	return selfAugmentResearchInfluences()
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return domain.ResearchInfluences()
}

func StepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return stepDurationStatByLabel(stats)
}

func StepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return stepDurationStatsForCompare(summary)
}

func SummarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return summarizeSelfAugment(result)
}

func SummarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return summarizeSelfVerification(result, targetScore)
}
