package selfverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestContractV8FencesV7GoalEvidence(t *testing.T) {
	// Given: the v7 contract carried the removed daemon resilience goal.
	current := ContractValue()
	previous := current
	previous.Version = 7
	previous.Hash = ""
	data, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	previous.Hash = hex.EncodeToString(sum[:])

	// Then: downstream hash/version checks cannot reuse v7 as a v8 baseline.
	if current.Version != 8 || current.Hash == previous.Hash {
		t.Fatalf("goal contract did not change: current=%+v previous=%+v", current, previous)
	}
	for _, name := range current.GoalNames {
		if name == "daemon_resilience" {
			t.Fatalf("v8 contract still names the removed daemon goal: %+v", current.GoalNames)
		}
	}
}
