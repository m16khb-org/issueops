package issueopsbenchmark

import (
	contract "issueops/internal/contract/issueopsbenchmark"
	routing "issueops/internal/domain/issueopsrouting"
)

// ScoreFixture scores one fixture's artifact, checking its routing trace
// against the fixture's expected routing.
func ScoreFixture(fixture contract.Fixture, artifact contract.Artifact) contract.Score {
	return ScoreArtifact(fixture, artifact, routing.Score(fixture.ExpectedRouting, artifact.RoutingTrace).OK)
}

// ScoreRun scores every fixture against the artifact stored under its ID in
// artifacts (a missing artifact scores as empty) and finalizes the run
// result under id.
func ScoreRun(id string, fixtures []contract.Fixture, artifacts map[string]contract.Artifact) contract.RunResult {
	result := contract.RunResult{ID: id, FixtureCount: len(fixtures)}
	for _, fixture := range fixtures {
		result.Scores = append(result.Scores, ScoreFixture(fixture, artifacts[fixture.ID]))
	}
	return FinalizeRun(result)
}
