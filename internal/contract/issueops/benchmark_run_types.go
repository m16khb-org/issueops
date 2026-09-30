package issueops

import benchmark "issueops/internal/contract/issueopsbenchmark"

type IssueOpsDimensionScore = benchmark.IssueOpsDimensionScore
type IssueOpsBenchmarkScore = benchmark.IssueOpsBenchmarkScore
type IssueOpsBenchmarkRunResult = benchmark.IssueOpsBenchmarkRunResult
type IssueOpsBenchmarkCompareResult = benchmark.IssueOpsBenchmarkCompareResult
type IssueOpsAutoresearchCandidate = benchmark.IssueOpsAutoresearchCandidate
type IssueOpsAutoresearchGateRequest = benchmark.IssueOpsAutoresearchGateRequest
type IssueOpsAutoresearchGateResult = benchmark.IssueOpsAutoresearchGateResult
type IssueOpsJudgeMap = benchmark.IssueOpsJudgeMap
type RecordedRun = benchmark.RecordedRun
type RecordedOutcomes = benchmark.RecordedOutcomes
type FixtureReliability = benchmark.FixtureReliability
type PassPowKPoint = benchmark.PassPowKPoint
type ReliabilityReport = benchmark.ReliabilityReport
type JudgeSample = benchmark.JudgeSample
type ConsensusVerdict = benchmark.ConsensusVerdict

// RoutingFidelityResult reports missing skill-at-phase pairings.
type RoutingFidelityResult struct {
	OK      bool           `json:"ok"`
	Missing []SkillRouting `json:"missing,omitempty"`
}
