package issueopsbenchmark

import (
	"strings"

	contract "issueops/internal/contract/issueopsbenchmark"
)

// FinalizeRun fills the derived fields of result from its scores: fixture and
// critical failure counts, average and minimum score, and OK, which holds only
// when there are no critical failures and every score passed.
func FinalizeRun(result contract.RunResult) contract.RunResult {
	result.FixtureCount = len(result.Scores)
	result.CriticalFailureCount = 0
	for _, score := range result.Scores {
		result.CriticalFailureCount += len(score.CriticalFailures)
	}
	result.AverageScore, result.MinimumScore = summarizeRunScores(result.Scores)
	result.OK = result.CriticalFailureCount == 0
	for _, score := range result.Scores {
		if !score.Passed {
			result.OK = false
			break
		}
	}
	return result
}

// MergeScoreWithJudge combines a deterministic score with a judge score. For
// each applicable dimension the lower of the two scores wins and the evidence
// is joined; judge failures and critical failures are appended, a judge with
// no dimension scores adds a failure, and Passed is recomputed.
func MergeScoreWithJudge(deterministic, judge contract.Score) contract.Score {
	merged := deterministic
	judgeByDimension := make(map[string]contract.IssueOpsDimensionScore)
	for _, score := range judge.DimensionScores {
		judgeByDimension[score.Dimension] = score
	}
	for i, score := range merged.DimensionScores {
		if score.NotApplicable {
			// N/A dimension은 채점에서 제외한다. judge score가 적용되지 않는
			// fixture에서 이를 다시 끌어들여서는 안 된다.
			continue
		}
		judgeScore, ok := judgeByDimension[score.Dimension]
		if !ok {
			continue
		}
		if judgeScore.Score < score.Score {
			merged.DimensionScores[i].Score = judgeScore.Score
		}
		merged.DimensionScores[i].Evidence = strings.TrimSpace(score.Evidence + "; judge: " + judgeScore.Evidence)
	}
	merged.JudgeFailures = append(merged.JudgeFailures, judge.JudgeFailures...)
	merged.CriticalFailures = append(merged.CriticalFailures, judge.CriticalFailures...)
	if len(judge.DimensionScores) == 0 {
		merged.JudgeFailures = append(merged.JudgeFailures, "judge returned no dimension scores")
	}
	merged.AverageScore, merged.MinimumScore = summarizeDimensionScores(merged.DimensionScores)
	merged.Passed = len(merged.CriticalFailures) == 0 && len(merged.DeterministicFailures) == 0 && len(merged.JudgeFailures) == 0 && merged.MinimumScore >= 100.0
	merged.OK = merged.Passed
	return merged
}
