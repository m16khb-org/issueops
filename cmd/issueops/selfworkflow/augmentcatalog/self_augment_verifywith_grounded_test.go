package augmentcatalog

import (
	app "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	"issueops/internal/testsupport"
	"slices"
	"strings"
	"testing"
)

// Every self-augment candidate (base + refilled quality specs) must carry an
// explicit verification kind and a VerifyWith that NAMES an external mechanism
// appropriate to that kind — never model self-critique (B1 self-correction
// guardrail). Before B1 only the 10 refilled quality candidates were checked
// for non-empty VerifyWith; the 21 base candidates had no grounding enforcement.
func TestEverySelfAugmentCandidateVerifyWithIsGrounded(t *testing.T) {
	candidates := app.Candidates(contract.SelfAugmentRepoSignals{})
	if len(candidates) < 20 {
		t.Fatalf("expected the full candidate catalog, got %d", len(candidates))
	}
	for _, candidate := range candidates {
		if candidate.VerificationKind == "" {
			t.Fatalf("candidate %q has no verification kind", candidate.ID)
		}
		if err := testsupport.VerifyWithGrounded(candidate.VerificationKind, candidate.VerifyWith); err != nil {
			t.Fatalf("candidate %q VerifyWith not grounded: %v", candidate.ID, err)
		}
	}
}

func TestSelfAugmentCandidateVerifyWithUsesLiveSelfVerifyFlags(t *testing.T) {
	for _, candidate := range app.Candidates(contract.SelfAugmentRepoSignals{}) {
		for _, mechanism := range candidate.VerifyWith {
			if !strings.Contains(mechanism, "self-verify") {
				continue
			}
			if strings.Contains(mechanism, "--full") || strings.Contains(mechanism, "--iterations") {
				t.Errorf("candidate %q suggests retired self-verify flags: %q", candidate.ID, mechanism)
			}
			if strings.Contains(mechanism, "--target-score") && !strings.Contains(mechanism, "--llm-eval=false") {
				t.Errorf("candidate %q target-score verification is not explicitly deterministic: %q", candidate.ID, mechanism)
			}
		}
	}
}

func TestAdapterContractMatrixCandidateCoversEveryFirstPartyHost(t *testing.T) {
	for _, candidate := range app.Candidates(contract.SelfAugmentRepoSignals{}) {
		if candidate.ID != "adapter-contract-matrix" {
			continue
		}
		for _, host := range []string{"Codex", "Claude", "Omo"} {
			if !strings.Contains(candidate.Title, host) {
				t.Errorf("adapter contract matrix title omits %s: %q", host, candidate.Title)
			}
		}
		want := "go test ./internal/adapter -run TestNativeInstallAdapterContractMatrix -count=1"
		if !slices.Contains(candidate.VerifyWith, want) {
			t.Errorf("adapter contract matrix VerifyWith=%q want %q", candidate.VerifyWith, want)
		}
		return
	}
	t.Fatal("adapter-contract-matrix candidate missing")
}
