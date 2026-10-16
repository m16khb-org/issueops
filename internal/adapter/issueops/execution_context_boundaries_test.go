package issueops

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"issueops/internal/testsupport"
)

func ownerArtifactPath(t *testing.T) (root, path string) {
	t.Helper()
	root = t.TempDir()
	return root, filepath.Join(root, ".issueops", "state", "artifact.bin")
}

func TestWriteSealedArtifactAcceptsExactlyTheArtifactSizeLimit(t *testing.T) {
	t.Parallel()

	root, path := ownerArtifactPath(t)
	atLimit := bytes.Repeat([]byte("a"), leasecontract.OwnerArtifactMaxBytes)
	if err := writeSealedArtifact(root, path, atLimit); err != nil {
		t.Fatalf("artifact of exactly the limit must be written: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, atLimit) {
		t.Fatalf("limit-sized artifact must round-trip: err=%v len=%d", err, len(got))
	}

	root, path = ownerArtifactPath(t)
	if err := writeSealedArtifact(root, path, append(atLimit, 'a')); err == nil {
		t.Fatal("artifact one byte over the limit must be refused")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("oversized artifact must not touch disk: %v", err)
	}
	if err := writeSealedArtifact(root, path, nil); err == nil {
		t.Fatal("empty artifact must be refused")
	}
}

func TestObserveNativeProcessReceiptRejectsNonPositivePID(t *testing.T) {
	t.Parallel()

	_, err := ObserveNativeProcessReceipt(0)
	if err == nil || err.Error() != "native process pid must be positive" {
		t.Fatalf("pid 0 error = %v", err)
	}
}

func TestObserveNativeProcessAncestryRejectsNonPositivePID(t *testing.T) {
	t.Parallel()

	_, err := ObserveNativeProcessAncestry(0)
	if err == nil || err.Error() != "native process pid must be positive" {
		t.Fatalf("pid 0 error = %v", err)
	}
}

func TestParseNativeProcessSnapshotRejectsPIDZeroRow(t *testing.T) {
	t.Parallel()

	_, err := parseNativeProcessSnapshot("0 1 Tue Jul 22 08:00:00 2026 /sbin/kernel_task\n")
	if err == nil || !strings.Contains(err.Error(), "invalid native process snapshot pid") {
		t.Fatalf("pid 0 row error = %v", err)
	}
}

func TestExecutionSyncBasePreviewReportsLeaseGeneration(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "9101-lease-generation")
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
	if err != nil {
		t.Fatal(err)
	}
	if want := fixture.record.Execution.Lease.Generation; want == 0 || result.LeaseGeneration != want {
		t.Fatalf("LeaseGeneration = %d, want %d", result.LeaseGeneration, want)
	}
}

func TestExecutionSyncBasePreviewDetectsRebaseDirectoryInProgress(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newReleasedSyncBaseFixture(t, "9102-"+name)
			if err := os.Mkdir(filepath.Join(fixture.git.gitDir, name), 0o700); err != nil {
				t.Fatal(err)
			}
			result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
			if err != nil {
				t.Fatal(err)
			}
			if !result.MergeInProgress {
				t.Fatalf("%s directory must count as an operation in progress: %+v", name, result)
			}
		})
	}
}

func TestExecutionSyncBaseApplyGateSeesShortestStatusLines(t *testing.T) {
	t.Parallel()

	fixture := newReleasedSyncBaseFixture(t, "9103-short-status")
	fixture.git.statusOut = " M a\n?? b"
	result, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBaseApply))
	if err == nil || !slices.Contains(result.Missing, "worktree_clean") {
		t.Fatalf("one-character tracked path must dirty the worktree: err=%v missing=%v", err, result.Missing)
	}
	if !slices.Equal(result.UntrackedWarnings, []string{"b"}) {
		t.Fatalf("one-character untracked path must be listed: %v", result.UntrackedWarnings)
	}
}

func TestExecutionSyncBaseEventActorLabelNamesHostSessionAndAgent(t *testing.T) {
	t.Parallel()

	for name, agent := range map[string]string{"without agent": "", "with agent": " worker-7 "} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fixture := newReleasedSyncBaseFixture(t, "9104-actor-label")
			fixture.actor.AgentID = agent
			preview, err := fixture.run(t, fixture.request(issueops.ExecutionSyncBasePreview))
			if err != nil {
				t.Fatal(err)
			}
			fixture.git.postMergeHead = syncBaseMergeOID
			req := fixture.request(issueops.ExecutionSyncBaseApply)
			req.Confirm, req.Fingerprint = true, preview.Fingerprint
			if _, err := fixture.run(t, req); err != nil {
				t.Fatal(err)
			}
			persisted, err := ReadIssueOps(fixture.stateRoot, fixture.record.ID)
			if err != nil {
				t.Fatal(err)
			}
			want := "codex/sync-base-9104-actor-label"
			if agent != "" {
				want += "/worker-7"
			}
			if events := persisted.Execution.SyncBaseEvents; len(events) != 1 || events[0].Actor != want {
				t.Fatalf("event actor = %#v, want %q", events, want)
			}
		})
	}
}

func TestDefaultExecutionSyncBaseGitReturnsStderrOnlyWhenStdoutIsEmpty(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := t.TempDir()
	if code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "init", "-q"); code != 0 {
		t.Fatalf("git init: %d %s", code, out)
	}

	if code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "rev-parse", "--git-dir"); code != 0 || strings.TrimSpace(out) != ".git" {
		t.Fatalf("success must return stdout: %d %q", code, out)
	}
	if code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "rev-parse", "--verify", "no-such-ref"); code == 0 || !strings.Contains(out, "fatal") {
		t.Fatalf("failure without stdout must return stderr: %d %q", code, out)
	}

	a, b := filepath.Join(repo, "a.txt"), filepath.Join(repo, "b.txt")
	if err := os.WriteFile(a, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "diff", "--no-index", "--exit-code", a, b)
	if code != 1 || !strings.Contains(out, "+two") {
		t.Fatalf("non-zero exit with stdout must return stdout: %d %q", code, out)
	}
}
