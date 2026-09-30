package benchmarkcmd

import (
	adapter "issueops/internal/adapter/issueops/benchmark"
	contract "issueops/internal/contract/issueopsbenchmark"
	"os"
)

func readIssueOpsJudgeMap(path string, fixtures []contract.IssueOpsBenchmarkFixture) (contract.IssueOpsJudgeMap, map[string]contract.IssueOpsBenchmarkScore, error) {
	m, err := (adapter.Files{Stdin: os.Stdin}).ReadJudgeMap(path, fixtures)
	return m, m.Scores, err
}
func readIssueOpsAutoresearchCandidateFile(path string) (contract.IssueOpsAutoresearchCandidate, error) {
	return (adapter.Files{}).ReadCandidate(path)
}
