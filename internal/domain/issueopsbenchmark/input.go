package issueopsbenchmark

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopsbenchmark"
)

func ValidateFixture(f contract.IssueOpsBenchmarkFixture) error {
	if strings.TrimSpace(f.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(f.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(f.UserPrompt) == "" {
		return fmt.Errorf("user_prompt is required")
	}
	if strings.TrimSpace(f.RepoContext) == "" {
		return fmt.Errorf("repo_context is required")
	}
	if len(f.CriticalFailures) == 0 {
		return fmt.Errorf("critical_failures is required")
	}
	return nil
}

func ValidateJudgeScore(score contract.IssueOpsBenchmarkScore) error {
	if len(score.DimensionScores) == 0 {
		return fmt.Errorf("issueops benchmark host-agent judge output missing dimension_scores")
	}
	return nil
}

// ResolveJudgeScores validates fixture coverage in fixture order. The decoder
// handles each opaque payload before the next fixture is checked, preserving
// malformed-score versus missing-fixture error precedence.
func ResolveJudgeScores(fixtures []contract.IssueOpsBenchmarkFixture, payloads map[string][]byte, decode func([]byte) (contract.IssueOpsBenchmarkScore, error)) (map[string]contract.IssueOpsBenchmarkScore, error) {
	scores := make(map[string]contract.IssueOpsBenchmarkScore, len(payloads))
	known := make(map[string]bool, len(fixtures))
	for _, fixture := range fixtures {
		known[fixture.ID] = true
		payload, ok := payloads[fixture.ID]
		if !ok {
			return nil, fmt.Errorf("judge map missing fixture %q", fixture.ID)
		}
		score, err := decode(payload)
		if err != nil {
			return nil, fmt.Errorf("judge score for fixture %q: %w", fixture.ID, err)
		}
		scores[fixture.ID] = score
	}
	for key := range payloads {
		if !known[key] {
			return nil, fmt.Errorf("judge map has unknown fixture %q", key)
		}
	}
	return scores, nil
}
