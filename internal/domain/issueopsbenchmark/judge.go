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
