package benchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	"issueops/internal/domain/judgement"
)

func DecodeJudgeScore(out []byte) (contract.Score, error) {
	var score contract.Score
	if err := judgement.DecodeStructuredJSONObject("issueops benchmark host-agent judge", out, &score); err != nil {
		return contract.Score{}, err
	}
	if err := domain.ValidateJudgeScore(score); err != nil {
		return contract.Score{}, err
	}
	return score, nil
}
