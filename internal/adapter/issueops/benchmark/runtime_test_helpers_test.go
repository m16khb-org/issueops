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
func SaveIssueOpsBenchmarkRun(root string, run IssueOpsBenchmarkRunResult) error {
	return (Store{Directory: root}).Save(run)
}
func ReadIssueOpsBenchmarkRun(root, id string) (IssueOpsBenchmarkRunResult, error) {
	return (Store{Directory: root}).Read(id)
}
func FinalizeIssueOpsBenchmarkRunResult(result IssueOpsBenchmarkRunResult) IssueOpsBenchmarkRunResult {
	return domain.FinalizeRun(result)
}
func MergeIssueOpsBenchmarkScoreWithJudge(deterministic, judge IssueOpsBenchmarkScore) IssueOpsBenchmarkScore {
	return domain.MergeScoreWithJudge(deterministic, judge)
}
func CompareIssueOpsBenchmarkRuns(baseline, candidate IssueOpsBenchmarkRunResult) IssueOpsBenchmarkCompareResult {
	return domain.CompareRuns(baseline, candidate)
}
func DecodeIssueOpsBenchmarkJudgeJSON(raw []byte) (IssueOpsBenchmarkScore, error) {
	return DecodeJudgeScore(raw)
}

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (IssueOpsBenchmarkRunResult, error) {
	result := domain.ScoreRun("issueops-benchmark-"+time.Now().UTC().Format("20060102T150405.000000000Z"), req.Fixtures, req.Artifacts)
	if strings.TrimSpace(req.StateRoot) != "" {
		if err := (Store{Directory: req.StateRoot}).Save(result); err != nil {
			return IssueOpsBenchmarkRunResult{}, err
		}
	}
	return result, nil
}

type testRunReader struct {
	read func(string) (IssueOpsBenchmarkRunResult, error)
}

func (r testRunReader) Read(id string) (IssueOpsBenchmarkRunResult, error) { return r.read(id) }
func (r testRunReader) Save(run IssueOpsBenchmarkRunResult) error          { panic("unexpected save") }
func validateJudgeProvenance(judge IssueOpsJudgeMap, scored, root string, read func(string, string) (IssueOpsBenchmarkRunResult, error)) error {
	return (app.Service{Runs: testRunReader{read: func(id string) (IssueOpsBenchmarkRunResult, error) { return read(root, id) }}}).ValidateJudgeProvenance(judge, scored)
}
func JudgeDownwardOverrideRate(deterministic, judge IssueOpsBenchmarkScore) (float64, int) {
	return domain.JudgeDownwardOverrideRate(deterministic, judge)
}
