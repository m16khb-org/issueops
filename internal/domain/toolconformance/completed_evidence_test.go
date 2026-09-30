package toolconformance

import (
	"testing"

	contract "issueops/internal/contract/toolconformance"
)

func TestCompletedEpisodeRejectsMissingLiveProof(t *testing.T) {
	episode := contract.EpisodeReport{Status: contract.EpisodeCompleted, Host: "codex", FixtureID: "fixture"}
	expected := CompletedEpisodeExpectation{Host: "codex", Fixture: contract.Fixture{ID: "fixture"}}
	if ValidCompletedEpisode(episode, expected) {
		t.Fatal("completed label without live proof must not be accepted")
	}
}
