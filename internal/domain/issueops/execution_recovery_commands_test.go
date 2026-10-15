package issueops

import "testing"

func TestReplacementRecoveryCommandsQuoteIdentityAndBindGeneration(t *testing.T) {
	if got := ReplacementPreviewCommand("io-1", 2); got != "issueops execution replace --id 'io-1' --expected-generation 2 --preview" {
		t.Fatalf("preview=%s", got)
	}
	if got := ReplacementFinalizePreviewCommand("io-1", 2); got != "issueops execution replace --id 'io-1' --expected-generation 2 --finalize-preview" {
		t.Fatalf("finalize preview=%s", got)
	}
	if got := ReplacementPreviewCommand("io'1", 5); got != `issueops execution replace --id 'io'\''1' --expected-generation 5 --preview` {
		t.Fatalf("quoted preview=%s", got)
	}
	if got := ReplacementClaimCommand("io-1", 3); got != "issueops execution claim --id 'io-1' --generation 3 --claim-current-token" {
		t.Fatalf("claim=%s", got)
	}
}
