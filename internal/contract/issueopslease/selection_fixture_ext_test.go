package issueopslease_test

import (
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
)

func leaseSelectionFixture[M ~string](mode M) *leasecontract.Selection {
	selection := &leasecontract.Selection{
		RequestedMode: string(mode), ResolvedMode: string(mode),
		ReadinessFingerprint: strings.Repeat("f", 64), SelectedAt: "2026-08-03T00:00:00Z",
	}
	if mode == "orca" {
		selection.ProbeAttempted, selection.ProbeAvailable, selection.ProbeReady, selection.ProbeCode = true, true, true, "ready"
	} else {
		selection.ExplicitDirectReason = "test fixture"
	}
	return selection
}
