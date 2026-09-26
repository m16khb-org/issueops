package benchmark

import domain "issueops/internal/domain/issueopsbenchmark"

func ConsensusJudgeVerdict(samples []JudgeSample) (ConsensusVerdict, error) {
	return domain.ConsensusJudgeVerdict(samples)
}
