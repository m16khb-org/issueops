package selfverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestContractV7FencesLegacyTimingEvidence(t *testing.T) {
	// Given: the v6 contract shape is unchanged apart from its version.
	current := ContractValue()
	legacy := current
	legacy.Version = 6
	legacy.Hash = ""
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	legacy.Hash = hex.EncodeToString(sum[:])

	// Then: downstream hash/version checks cannot reuse v6 as a v7 baseline.
	if current.Version != 7 || current.Hash == legacy.Hash {
		t.Fatalf("timing contract did not change: current=%+v legacy=%+v", current, legacy)
	}
}
