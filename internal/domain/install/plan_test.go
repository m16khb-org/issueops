package install

import "testing"

func TestCandidatePathRequiresCanonicalOrStagedSiblingForApply(t *testing.T) {
	target := "/root/bin/issueops"
	if !CandidatePathAllowed(target, target, false) || !CandidatePathAllowed("/root/bin/.issueops.activate-abc", target, false) {
		t.Fatal("valid apply candidate rejected")
	}
	if CandidatePathAllowed("/elsewhere/issueops", target, false) || CandidatePathAllowed("/root/bin/other", target, false) {
		t.Fatal("external or unrelated apply candidate accepted")
	}
	if !CandidatePathAllowed("/elsewhere/issueops", target, true) {
		t.Fatal("dry-run candidate should not be constrained")
	}
}

func TestActivationStepRejectsDryRunMutationAndUnknownStep(t *testing.T) {
	if _, err := ActivationStep(true, "begin"); err == nil {
		t.Fatal("dry-run activation accepted")
	}
	if _, err := ActivationStep(false, "unknown"); err == nil {
		t.Fatal("unknown activation step accepted")
	}
	if got, err := ActivationStep(false, " seal "); err != nil || got != "seal" {
		t.Fatalf("trimmed seal step = %q, %v", got, err)
	}
}

func TestPathModeAllowlist(t *testing.T) {
	for _, mode := range []string{"auto", "manual", "skip"} {
		if !ValidPathMode(mode) {
			t.Fatalf("mode %q rejected", mode)
		}
	}
	if ValidPathMode("all") {
		t.Fatal("unknown path mode accepted")
	}
}
