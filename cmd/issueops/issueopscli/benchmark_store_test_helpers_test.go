package issueopscli

import (
	adapter "issueops/internal/adapter/issueops/benchmark"
	contract "issueops/internal/contract/issueopsbenchmark"
)

func saveBenchmarkRunForTest(root string, run contract.RunResult) error {
	return (adapter.Store{Directory: root}).Save(run)
}
