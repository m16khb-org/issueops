package issueopsbenchmark

import contract "issueops/internal/contract/issueopsbenchmark"

func CompareRuns(baseline, candidate contract.IssueOpsBenchmarkRunResult) contract.IssueOpsBenchmarkCompareResult {
	result := contract.IssueOpsBenchmarkCompareResult{
		OK:                   true,
		BaselineID:           baseline.ID,
		CandidateID:          candidate.ID,
		AverageScoreDelta:    candidate.AverageScore - baseline.AverageScore,
		MinimumScoreDelta:    candidate.MinimumScore - baseline.MinimumScore,
		CriticalFailureDelta: candidate.CriticalFailureCount - baseline.CriticalFailureCount,
		Regressions:          compareIssueOpsDimensionRegressions(baseline, candidate),
	}
	result.Improved = result.AverageScoreDelta > 0 &&
		result.MinimumScoreDelta >= 0 &&
		result.CriticalFailureDelta <= 0 &&
		len(result.Regressions) == 0
	result.OK = result.MinimumScoreDelta >= 0 &&
		result.CriticalFailureDelta <= 0 &&
		len(result.Regressions) == 0
	return result
}
