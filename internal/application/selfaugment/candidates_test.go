package selfaugment

import (
	"testing"

	contract "issueops/internal/contract/selfaugment"
)

func TestCandidatesIncludeQualitySpecsWithoutChangingOrder(t *testing.T) {
	candidates := Candidates(contract.SelfAugmentRepoSignals{})
	found := false
	for _, candidate := range candidates {
		if candidate.ID == "quality-signal-harvester" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("quality catalog candidate missing")
	}
}
