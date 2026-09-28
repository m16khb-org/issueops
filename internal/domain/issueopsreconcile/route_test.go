package issueopsreconcile

import "testing"

func TestDecidePendingPreservesPreviewAndConfirmRouting(t *testing.T) {
	for _, tt := range []struct {
		name, mode, kind string
		pending          bool
		wantRoute        Route
		wantPreview      string
		wantSkipGuard    bool
	}{
		{name: "no intent", mode: "orca", wantRoute: RouteNone, wantPreview: "no_pending_external_intent"},
		{name: "remote PR", mode: "orca", kind: "remote_pr_create", pending: true, wantRoute: RouteRemotePR, wantPreview: "remote_reconcile_required"},
		{name: "Orca owner", mode: "orca", kind: "owner_launch", pending: true, wantRoute: RouteOrca, wantPreview: "orca_reconcile_required", wantSkipGuard: true},
		{name: "direct with Orca intent", mode: "direct", kind: "dispatch", pending: true, wantRoute: RouteOrca, wantPreview: "orca_reconcile_required"},
		{name: "unsupported", mode: "orca", kind: "future", pending: true, wantRoute: RouteUnsupported, wantPreview: "unsupported_external_intent"},
		{name: "padded intent", mode: "orca", kind: " owner_launch ", pending: true, wantRoute: RouteUnsupported, wantPreview: "orca_reconcile_required", wantSkipGuard: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := DecidePending(tt.mode, tt.kind, tt.pending)
			if got.Route != tt.wantRoute || got.PreviewCode != tt.wantPreview || got.SkipMutationGuard != tt.wantSkipGuard {
				t.Fatalf("decision=%+v want route=%s preview=%q skip=%v", got, tt.wantRoute, tt.wantPreview, tt.wantSkipGuard)
			}
		})
	}
}

func TestIsOrcaIntentKindKeepsCleanupAllowlist(t *testing.T) {
	for _, kind := range []string{"worktree_create", "owner_launch", "dispatch", " owner_launch "} {
		if !IsOrcaIntentKind(kind) {
			t.Fatalf("Orca kind %q rejected", kind)
		}
	}
	for _, kind := range []string{"", "remote_pr_create", "future"} {
		if IsOrcaIntentKind(kind) {
			t.Fatalf("non-Orca kind %q accepted", kind)
		}
	}
}
