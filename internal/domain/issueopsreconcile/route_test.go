package issueopsreconcile

import "testing"

func TestDecidePendingPreservesPreviewAndConfirmRouting(t *testing.T) {
	for _, tt := range []struct {
		name, kind  string
		pending     bool
		wantRoute   Route
		wantPreview string
	}{
		{name: "no intent", wantRoute: RouteNone, wantPreview: "no_pending_external_intent"},
		{name: "remote PR", kind: "remote_pr_create", pending: true, wantRoute: RouteRemotePR, wantPreview: "remote_reconcile_required"},
		{name: "unsupported", kind: "future", pending: true, wantRoute: RouteUnsupported, wantPreview: "unsupported_external_intent"},
		{name: "padded remote PR", kind: " remote_pr_create ", pending: true, wantRoute: RouteUnsupported, wantPreview: "unsupported_external_intent"},
		{name: "empty kind", pending: true, wantRoute: RouteUnsupported, wantPreview: "unsupported_external_intent"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := DecidePending(tt.kind, tt.pending)
			if got.Route != tt.wantRoute || got.PreviewCode != tt.wantPreview {
				t.Fatalf("decision=%+v want route=%s preview=%q", got, tt.wantRoute, tt.wantPreview)
			}
		})
	}
}
