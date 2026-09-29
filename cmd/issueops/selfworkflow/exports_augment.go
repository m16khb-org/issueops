package selfworkflow

import (
	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	"issueops/cmd/issueops/selfworkflow/augmentplan"
	domain "issueops/internal/domain/selfaugment"
)

func allSelfAugmentGoalsPassed(goals []SelfAugmentGoal) bool {
	return augmentcatalog.AllSelfAugmentGoalsPassed(goals)
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
	return augmentcatalog.CollectSelfAugmentRepoSignals(root, docsIndexed, skills, geniusText)
}

func DocsContainTerm(root, term string) bool {
	return docsContainTerm(root, term)
}

func docsContainTerm(root, term string) bool {
	return augmentcatalog.DocsContainTerm(root, term)
}

func DirContainsTerm(root, relDir, term string) bool {
	return dirContainsTerm(root, relDir, term)
}

func dirContainsTerm(root, relDir, term string) bool {
	return augmentcatalog.DirContainsTerm(root, relDir, term)
}

func FileContainsTerm(root, relPath, term string) bool {
	return fileContainsTerm(root, relPath, term)
}

func fileContainsTerm(root, relPath, term string) bool {
	return augmentcatalog.FileContainsTerm(root, relPath, term)
}

func FormatScore(score float64) string {
	return formatScore(score)
}

func MarkSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	markSatisfiedSelfAugmentCandidate(candidate, signals)
}

func markSatisfiedSelfAugmentCandidate(candidate *SelfAugmentCandidate, signals SelfAugmentRepoSignals) {
	augmentcatalog.MarkSatisfiedSelfAugmentCandidate(candidate, signals)
}

func MaxSlowStepDurationByLabel(steps []SelfAugmentSlowStep) map[string]int64 {
	return maxSlowStepDurationByLabel(steps)
}

func PlanSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return planSelfAugmentation(req)
}

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return augmentplan.Plan(req, IssueOpsRoot(), Version)
}

func ScoreBool(ok bool) float64 {
	return scoreBool(ok)
}

func scoreBool(ok bool) float64 {
	return augmentcatalog.ScoreBool(ok)
}

func SaveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	return saveSelfAugmentLesson(req)
}

func ScoreSelfVerificationGoals(result SelfAugmentResult, targetScore float64) []SelfVerificationGoalScore {
	return scoreSelfVerificationGoals(result, targetScore)
}

func SelectGeniusFormulas(text string) []string {
	return selectGeniusFormulas(text)
}

func selectGeniusFormulas(text string) []string {
	return augmentcatalog.SelectGeniusFormulas(text)
}

func SelectedCandidateID(candidate *SelfAugmentCandidate) string {
	return selectedCandidateID(candidate)
}

func selectedCandidateID(candidate *SelfAugmentCandidate) string {
	return augmentcatalog.SelectedCandidateID(candidate)
}

func SelfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return selfAugmentCandidates(signals)
}

func selfAugmentCandidates(signals SelfAugmentRepoSignals) []SelfAugmentCandidate {
	return augmentcatalog.SelfAugmentCandidates(signals)
}

func SelfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return selfAugmentCandidateScore(candidate)
}

func selfAugmentCandidateScore(candidate SelfAugmentCandidate) float64 {
	return augmentcatalog.SelfAugmentCandidateScore(candidate)
}

func SelfAugmentCandidateIDsByStatus(candidates []SelfAugmentCandidate, status string) []string {
	return domain.CandidateIDsByStatus(candidates, status)
}

func SelfAugmentResearchInfluences() []SelfAugmentInfluence {
	return selfAugmentResearchInfluences()
}

func selfAugmentResearchInfluences() []SelfAugmentInfluence {
	return augmentcatalog.SelfAugmentResearchInfluences()
}

func StepDurationStatByLabel(stats []SelfAugmentStepDurationStat) map[string]SelfAugmentStepDurationStat {
	return stepDurationStatByLabel(stats)
}

func StepDurationStatsForCompare(summary SelfAugmentSummary) []SelfAugmentStepDurationStat {
	return stepDurationStatsForCompare(summary)
}

func RunSelfAugmentLesson(args []string) error {
	return runSelfAugmentLesson(args)
}

func StateKeySlug(s string) string {
	return stateKeySlug(s)
}

func SummarizeSelfAugment(result SelfAugmentResult) SelfAugmentSummary {
	return summarizeSelfAugment(result)
}

func SummarizeSelfVerification(result SelfAugmentResult, targetScore float64) SelfAugmentSummary {
	return summarizeSelfVerification(result, targetScore)
}
