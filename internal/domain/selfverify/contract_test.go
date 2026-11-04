package selfverify

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"testing"
)

func TestContractV9FencesV8SummaryWithoutNotRunMarkers(t *testing.T) {
	// Given: the v8 contract had no planned/not-run fields and still named no daemon goal.
	current := ContractValue()
	previous := current
	previous.Version = 8
	previous.Hash = ""
	previous.RequiredFields = slices.DeleteFunc(slices.Clone(current.RequiredFields), func(field string) bool {
		return field == "planned_steps" || field == "not_run_steps"
	})
	data, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	previous.Hash = hex.EncodeToString(sum[:])

	// Then: downstream hash/version checks cannot reuse v8 as a v9 baseline.
	if current.Version != 9 || current.Hash == previous.Hash {
		t.Fatalf("summary contract did not change: current=%+v previous=%+v", current, previous)
	}
	for _, field := range []string{"planned_steps", "not_run_steps"} {
		if !slices.Contains(current.RequiredFields, field) {
			t.Fatalf("v9 contract does not require %s: %v", field, current.RequiredFields)
		}
	}
	for _, name := range current.GoalNames {
		if name == "daemon_resilience" {
			t.Fatalf("v9 contract still names the removed daemon goal: %+v", current.GoalNames)
		}
	}
}
