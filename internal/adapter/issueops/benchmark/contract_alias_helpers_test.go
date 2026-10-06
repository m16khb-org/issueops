package benchmark

import (
	issueopscontract "issueops/internal/contract/issueops"
	benchmarkcontract "issueops/internal/contract/issueopsbenchmark"
)

// 벤치마크 DTO는 계약이 소유한다. 어댑터는 같은 이름으로 재노출만 한다.
type (
	IssueOpsDimensionScore          = benchmarkcontract.IssueOpsDimensionScore
	IssueOpsBenchmarkScore          = benchmarkcontract.IssueOpsBenchmarkScore
	IssueOpsBenchmarkRunResult      = benchmarkcontract.IssueOpsBenchmarkRunResult
	IssueOpsBenchmarkCompareResult  = benchmarkcontract.IssueOpsBenchmarkCompareResult
	IssueOpsAutoresearchCandidate   = benchmarkcontract.IssueOpsAutoresearchCandidate
	IssueOpsAutoresearchGateRequest = benchmarkcontract.IssueOpsAutoresearchGateRequest
	IssueOpsAutoresearchGateResult  = benchmarkcontract.IssueOpsAutoresearchGateResult
	IssueOpsJudgeMap                = benchmarkcontract.IssueOpsJudgeMap
	RecordedRun                     = benchmarkcontract.RecordedRun
	RecordedOutcomes                = benchmarkcontract.RecordedOutcomes
	FixtureReliability              = benchmarkcontract.FixtureReliability
	PassPowKPoint                   = benchmarkcontract.PassPowKPoint
	ReliabilityReport               = benchmarkcontract.ReliabilityReport
	RoutingFidelityResult           = issueopscontract.RoutingFidelityResult
	JudgeSample                     = benchmarkcontract.JudgeSample
	ConsensusVerdict                = benchmarkcontract.ConsensusVerdict
)

// Test fixture for the retired adapter runner; production uses application.Service.
type IssueOpsBenchmarkRunRequest struct {
	StateRoot string
	Fixtures  []issueopscontract.IssueOpsBenchmarkFixture
	Artifacts map[string]issueopscontract.IssueOpsBenchmarkArtifact
}
