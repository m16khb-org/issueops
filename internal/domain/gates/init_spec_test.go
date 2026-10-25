package gates

import (
	"strings"
	"testing"

	model "issueops/internal/contract/gates"
)

// Probe: `gates init --gate nonsense` created a gate whose id became line3,
// and "CHECK:" with nothing after it wrote an empty command.
func TestRenderInitialLedgerRefusesIDlessAndEmptyCheckSpecs(t *testing.T) {
	for spec, want := range map[string]string{
		"nonsense":                 "start with an id",
		"tests pass | CHECK: true": "start with an id",
		"G1: a | CHECK:":           "CHECK is empty",
	} {
		if _, err := RenderInitialLedger(model.InitRequest{Scope: "x", Gates: []string{spec}}); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%q: err = %v", spec, err)
		}
	}
	if _, err := RenderInitialLedger(model.InitRequest{Scope: "x", Gates: []string{"G2: manual outcome", "G1: a | CHECK: true | EXPECT:"}}); err != nil {
		t.Fatalf("valid specs refused: %v", err)
	}
}
