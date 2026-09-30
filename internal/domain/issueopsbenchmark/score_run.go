package issueopsbenchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	routing "issueops/internal/domain/issueopsrouting"
)

func ScoreFixture(fixture contract.IssueOpsBenchmarkFixture, artifact contract.IssueOpsBenchmarkArtifact) contract.IssueOpsBenchmarkScore {
	return ScoreArtifact(fixture, artifact, routing.Score(fixture.ExpectedRouting, artifact.RoutingTrace).OK)
}

func ScoreRun(id string, fixtures []contract.IssueOpsBenchmarkFixture, artifacts map[string]contract.IssueOpsBenchmarkArtifact) contract.IssueOpsBenchmarkRunResult {
	result := contract.IssueOpsBenchmarkRunResult{ID: id, FixtureCount: len(fixtures)}
	for _, fixture := range fixtures {
		result.Scores = append(result.Scores, ScoreFixture(fixture, artifacts[fixture.ID]))
	}
	return FinalizeRun(result)
}
