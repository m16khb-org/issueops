package issueopsbenchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	routing "issueops/internal/domain/issueopsrouting"
)

func ScoreFixture(fixture contract.Fixture, artifact contract.Artifact) contract.Score {
	return ScoreArtifact(fixture, artifact, routing.Score(fixture.ExpectedRouting, artifact.RoutingTrace).OK)
}

func ScoreRun(id string, fixtures []contract.Fixture, artifacts map[string]contract.Artifact) contract.RunResult {
	result := contract.RunResult{ID: id, FixtureCount: len(fixtures)}
	for _, fixture := range fixtures {
		result.Scores = append(result.Scores, ScoreFixture(fixture, artifacts[fixture.ID]))
	}
	return FinalizeRun(result)
}
