package benchmark

import (
	issueopscontract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
)

// issueOpsBenchmarkDimensions is the live scoring vocabulary: ScoreArtifact
// records exactly one entry per dimension, scored or not applicable.
var issueOpsBenchmarkDimensions = func() []string {
	score := domain.ScoreArtifact(issueopscontract.IssueOpsBenchmarkFixture{}, issueopscontract.IssueOpsBenchmarkArtifact{}, false)
	out := make([]string, 0, len(score.DimensionScores))
	for _, dimension := range score.DimensionScores {
		out = append(out, dimension.Dimension)
	}
	return out
}()

const issueOpsBenchmarkMaxScore = 100.0
