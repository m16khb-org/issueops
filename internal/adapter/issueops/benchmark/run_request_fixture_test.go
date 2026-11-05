package benchmark

import (
	benchmarkcontract "issueops/internal/contract/issueopsbenchmark"
)

// Test fixture for the retired adapter runner; production uses application.Service.
type IssueOpsBenchmarkRunRequest struct {
	StateRoot string
	Fixtures  []benchmarkcontract.Fixture
	Artifacts map[string]benchmarkcontract.Artifact
}
