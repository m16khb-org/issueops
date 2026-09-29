package issueopscli

import (
	adapter "issueops/internal/adapter/issueops/benchmark"
	contract "issueops/internal/contract/issueopsbenchmark"
)

func saveBenchmarkRunForTest(root string, run contract.IssueOpsBenchmarkRunResult) error {
	return (adapter.Store{Directory: root}).Save(run)
}
func readBenchmarkRunForTest(root, id string) (contract.IssueOpsBenchmarkRunResult, error) {
	return (adapter.Store{Directory: root}).Read(id)
}
