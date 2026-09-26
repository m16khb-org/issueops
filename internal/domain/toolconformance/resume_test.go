package toolconformance

import (
	"testing"

	contract "issueops/internal/contract/toolconformance"
)

func TestValidatePreviousSelectionRejectsDuplicateHosts(t *testing.T) {
	previous := &contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "codex"}, {Host: "codex"}}}
	err := ValidatePreviousSelection(previous, []SelectedFixturePair{{Host: "codex", Fixture: contract.Fixture{ID: "fixture"}}})
	if err == nil || err.Error() != "invalid_previous_report_identity" {
		t.Fatalf("duplicate host accepted: %v", err)
	}
}
