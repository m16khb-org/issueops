package issueopsbenchmark

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func ValidateJudgeMetadata(judge contract.IssueOpsJudgeMap, scoredRunID string) (string, error) {
	sourceID := strings.TrimSpace(judge.SourceRunID)
	if sourceID == "" {
		return "", fmt.Errorf("judge map missing source_run_id (judge provenance is required for --judge file)")
	}
	if strings.TrimSpace(judge.Provenance) == "" {
		return "", fmt.Errorf("judge map missing provenance label (name how the judge scores were produced)")
	}
	if sourceID == strings.TrimSpace(scoredRunID) {
		return "", fmt.Errorf("judge map source_run_id %q is the scored run itself — a self-attributed judge map (one run dressed as a judge of itself) is rejected", sourceID)
	}
	return sourceID, nil
}

func JudgeDownwardOverrideRate(deterministic, judge contract.IssueOpsBenchmarkScore) (rate float64, comparable int) {
	judgeByDimension := make(map[string]float64, len(judge.DimensionScores))
	for _, dim := range judge.DimensionScores {
		judgeByDimension[dim.Dimension] = dim.Score
	}
	lowered := 0
	for _, dim := range deterministic.DimensionScores {
		if dim.NotApplicable {
			continue
		}
		judgeScore, ok := judgeByDimension[dim.Dimension]
		if !ok {
			continue
		}
		comparable++
		if judgeScore < dim.Score {
			lowered++
		}
	}
	if comparable == 0 {
		return 0, 0
	}
	return Round4(float64(lowered) / float64(comparable)), comparable
}
