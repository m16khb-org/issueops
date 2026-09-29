package gates

import (
	model "issueops/internal/contract/gates"
	"strings"
	"testing"
)

func TestInitAdmissionAndSpecRenderingPreserveOrder(t *testing.T) {
	req, result, err := PrepareInit(model.InitRequest{Scope: " 한글 scope ", Gates: []string{"G1: proof | CHECK: printf ok | EXPECT: ok"}})
	if err != nil || req.Scope != "한글 scope" || result.File != ".issueops/gates/한글-scope.md" {
		t.Fatalf("req=%+v result=%+v err=%v", req, result, err)
	}
	body, err := RenderInitialLedger(req)
	if err != nil || !strings.Contains(body, "EVIDENCE: pending") {
		t.Fatalf("body=%q err=%v", body, err)
	}
	_, partial, err := PrepareInit(model.InitRequest{File: " chosen.md ", Scope: "scope"})
	if err == nil || partial.File != "chosen.md" {
		t.Fatalf("partial=%+v err=%v", partial, err)
	}
	_, _, err = PrepareInit(model.InitRequest{Scope: "!!!"})
	if err == nil || err.Error() != "scope must contain a letter or number" {
		t.Fatalf("error precedence=%v", err)
	}
}
func TestRenderInitialLedgerRejectsUnquotedControlOperator(t *testing.T) {
	for _, spec := range []string{"G1: proof | CHECK: printf ok && touch marker", "G1: proof | WHAT: invalid", "| CHECK: printf ok"} {
		if _, err := RenderInitialLedger(model.InitRequest{Scope: "scope", Gates: []string{spec}}); err == nil {
			t.Fatalf("accepted %q", spec)
		}
	}
}
