package gitlab

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/port"
)

func TestVerifyChildHierarchyStopsOnCanceledContext(t *testing.T) {
	repo := installGlab(t, ": > ran\nprintf '{}'\n")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := NewProvider().VerifyChildHierarchy(ctx, port.IssueProviderChildHierarchyRequest{
		Repo: repo, ParentIssueURL: gapParentURL, ChildURL: gapChildURL,
	}); err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "ran")); statErr == nil {
		t.Fatal("glab ran although its context was canceled")
	}
}
