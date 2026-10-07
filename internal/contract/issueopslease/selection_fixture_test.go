package issueopslease

import (
	"strings"
)

func leaseSelectionFixture[M ~string](mode M) *Selection {
	selection := &Selection{
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
