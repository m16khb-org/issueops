package benchmark

import domain "issueops/internal/domain/issueopsbenchmark"

const maxReliabilityTrials = domain.MaxReliabilityTrials

func ComputeReliability(rec RecordedOutcomes, alpha float64) (ReliabilityReport, error) {
	return domain.ComputeReliability(rec, alpha)
}

func passPowK(successes, trials, k int) (float64, error) {
	return domain.PassPowK(successes, trials, k)
}

func clopperPearson(c, n int, alpha float64) (float64, float64, error) {
	return domain.ClopperPearson(c, n, alpha)
}

func ScoreSpread(scores []float64) (float64, float64, float64) {
	return domain.ScoreSpread(scores)
}
