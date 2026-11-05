//go:build unix

package issueops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/testsupport"
)

// canceledProviderCLI puts a fake gh on PATH that leaves a marker file when it
// runs, and returns a canceled context plus the marker path.
func canceledProviderCLI(t *testing.T) (context.Context, string) {
	t.Helper()
	bin := t.TempDir()
	testsupport.WriteFakeCommand(t, bin, "gh", "#!/bin/sh\n: > \"$(dirname \"$0\")/ran\"\n")
	t.Setenv("PATH", bin)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx, filepath.Join(bin, "ran")
}

func TestIssueGraphPosterStopsOnCanceledContext(t *testing.T) {
	ctx, marker := canceledProviderCLI(t)
	if _, err := (IssueGraphPoster{}).Post(ctx, "github", "https://github.com/acme/repo/issues/1", "body"); err == nil {
		t.Fatal("a canceled context must fail the post")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("gh ran although its context was canceled")
	}
}

func TestObserveRemoteArtifactStopsOnCanceledContext(t *testing.T) {
	ctx, marker := canceledProviderCLI(t)
	if _, err := ObserveRemoteArtifact(ctx, "https://github.com/acme/repo/pull/1"); err == nil {
		t.Fatal("a canceled context must fail the observation")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("gh ran although its context was canceled")
	}
}
