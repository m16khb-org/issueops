package github

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/port"
)

func TestVerifyChildHierarchyStopsOnCanceledContext(t *testing.T) {
	repo := fakeGhRepo(t, "#!/bin/sh\n: > ran\nprintf '[]'\n")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := NewProvider().VerifyChildHierarchy(ctx, port.IssueProviderChildHierarchyRequest{
		Repo: repo, ParentIssueURL: gapParentURL, ChildURL: "https://github.com/acme/repo/issues/31",
	})
	if err == nil {
		t.Fatal("expected the canceled context to fail the verification")
	}
	if _, statErr := os.Stat(filepath.Join(repo, "ran")); statErr == nil {
		t.Fatal("gh ran although its context was canceled")
	}
}
