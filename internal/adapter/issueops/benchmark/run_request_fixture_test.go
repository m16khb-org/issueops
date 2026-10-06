package benchmark

import (
	benchmarkcontract "issueops/internal/contract/issueopsbenchmark"
)

// Test fixture for the retired adapter runner; production uses application.Service.
type IssueOpsBenchmarkRunRequest struct {
	StateRoot string
	Fixtures  []benchmarkcontract.IssueOpsBenchmarkFixture
	Artifacts map[string]benchmarkcontract.IssueOpsBenchmarkArtifact
}
