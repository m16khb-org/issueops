package benchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	"issueops/internal/domain/judgement"
)

func DecodeJudgeScore(out []byte) (contract.IssueOpsBenchmarkScore, error) {
	var score contract.IssueOpsBenchmarkScore
	if err := judgement.DecodeStructuredJSONObject("issueops benchmark host-agent judge", out, &score); err != nil {
		return contract.IssueOpsBenchmarkScore{}, err
	}
	if err := domain.ValidateJudgeScore(score); err != nil {
		return contract.IssueOpsBenchmarkScore{}, err
	}
	return score, nil
}
