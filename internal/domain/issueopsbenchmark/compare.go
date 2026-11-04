// Package issueopsbenchmark is the domain layer of the issueops skill
// benchmark: it scores recorded agent artifacts against fixtures on fixed
// quality dimensions, merges host-agent judge scores, aggregates run results,
// compares runs for regressions, gates autoresearch candidates, and computes
// offline reliability statistics. It runs no agents and performs no I/O.
package issueopsbenchmark

import contract "issueops/internal/contract/issueopsbenchmark"

// CompareRuns compares candidate with baseline by average, minimum and
// critical-failure deltas and by per-dimension minimum scores. OK holds when
// the minimum did not drop, critical failures did not rise and no dimension
// regressed; Improved additionally requires a higher average.
func CompareRuns(baseline, candidate contract.RunResult) contract.CompareResult {
	result := contract.CompareResult{
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
