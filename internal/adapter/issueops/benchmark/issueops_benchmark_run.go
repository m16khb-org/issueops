package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	domain "issueops/internal/domain/issueopsbenchmark"
)

func RunIssueOpsBenchmark(req IssueOpsBenchmarkRunRequest) (IssueOpsBenchmarkRunResult, error) {
	result := IssueOpsBenchmarkRunResult{
		ID:           "issueops-benchmark-" + time.Now().UTC().Format("20060102T150405.000000000Z"),
		FixtureCount: len(req.Fixtures),
	}
	for _, fixture := range req.Fixtures {
		artifact := req.Artifacts[fixture.ID]
		score := ScoreIssueOpsBenchmarkArtifact(fixture, artifact)
		result.Scores = append(result.Scores, score)
		result.CriticalFailureCount += len(score.CriticalFailures)
	}
	result = FinalizeIssueOpsBenchmarkRunResult(result)
	if strings.TrimSpace(req.StateRoot) != "" {
		if err := SaveIssueOpsBenchmarkRun(req.StateRoot, result); err != nil {
			return IssueOpsBenchmarkRunResult{}, err
		}
	}
	return result, nil
}

func SaveIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	return persistIssueOpsBenchmarkRun(stateRoot, result)
}

func ReadIssueOpsBenchmarkRun(stateRoot, id string) (IssueOpsBenchmarkRunResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return IssueOpsBenchmarkRunResult{}, fmt.Errorf("benchmark id is required")
	}
	b, err := os.ReadFile(filepath.Join(stateRoot, "issueops-benchmarks", id+".json"))
	if err != nil {
		return IssueOpsBenchmarkRunResult{}, err
	}
	var result IssueOpsBenchmarkRunResult
	if err := json.Unmarshal(b, &result); err != nil {
		return IssueOpsBenchmarkRunResult{}, err
	}
	return result, nil
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

func persistIssueOpsBenchmarkRun(stateRoot string, result IssueOpsBenchmarkRunResult) error {
	dir := filepath.Join(stateRoot, "issueops-benchmarks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, result.ID+".json"), b, 0o644)
}
