package benchmark

import (
	app "issueops/internal/application/issueopsbenchmark"
	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	"strings"
	"time"
)

func LoadIssueOpsBenchmarkFixtures(path string) ([]contract.IssueOpsBenchmarkFixture, error) {
	return (Files{}).LoadFixtures(path)
}
func SaveIssueOpsBenchmarkRun(root string, run contract.IssueOpsBenchmarkRunResult) error {
	return (Store{Directory: root}).Save(run)
}
func ReadIssueOpsBenchmarkRun(root, id string) (contract.IssueOpsBenchmarkRunResult, error) {
	return (Store{Directory: root}).Read(id)
}

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (contract.IssueOpsBenchmarkRunResult, error) {
	result := domain.ScoreRun("issueops-benchmark-"+time.Now().UTC().Format("20060102T150405.000000000Z"), req.Fixtures, req.Artifacts)
	if strings.TrimSpace(req.StateRoot) != "" {
		if err := (Store{Directory: req.StateRoot}).Save(result); err != nil {
			return contract.IssueOpsBenchmarkRunResult{}, err
		}
	}
	return result, nil
}

type testRunReader struct {
	read func(string) (contract.IssueOpsBenchmarkRunResult, error)
}

func (r testRunReader) Read(id string) (contract.IssueOpsBenchmarkRunResult, error) {
	return r.read(id)
}
func (r testRunReader) Save(run contract.IssueOpsBenchmarkRunResult) error { panic("unexpected save") }
func validateJudgeProvenance(judge contract.IssueOpsJudgeMap, scored, root string, read func(string, string) (contract.IssueOpsBenchmarkRunResult, error)) error {
	return (app.Service{Runs: testRunReader{read: func(id string) (contract.IssueOpsBenchmarkRunResult, error) { return read(root, id) }}}).ValidateJudgeProvenance(judge, scored)
}
