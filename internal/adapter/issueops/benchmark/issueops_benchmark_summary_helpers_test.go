package benchmark

import domain "issueops/internal/domain/issueopsbenchmark"

func summarizeIssueOpsDimensionScores(scores []IssueOpsDimensionScore) (float64, float64) {
	return domain.SummarizeDimensionScores(scores)
}

func summarizeIssueOpsRunScores(scores []IssueOpsBenchmarkScore) (float64, float64) {
	return domain.SummarizeRunScores(scores)
}
