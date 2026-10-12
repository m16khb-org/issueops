package issueopslease

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

func newReseedGitRepo(t *testing.T, branch string) string {
	t.Helper()
	testsupport.IsolateGitConfig(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	reseedInventoryGit(t, root, "init", "--initial-branch", branch)
	reseedInventoryGit(t, root, "config", "user.email", "test@example.invalid")
	reseedInventoryGit(t, root, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	reseedInventoryGit(t, root, "add", "tracked.txt")
	reseedInventoryGit(t, root, "commit", "--message", "initial")
	return root
}

func TestReseedWorkspaceSnapshotFingerprintTracksEveryWorkspaceChannel(t *testing.T) {
	root := newReseedGitRepo(t, "snap")
	workspace := leasecontract.Workspace{Root: root, Branch: "snap"}
	snapshot := func() string {
		t.Helper()
		value, err := reseedWorkspaceSnapshot(workspace)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	clean := snapshot()
	if snapshot() != clean {
		t.Fatal("snapshot of an unchanged worktree must be deterministic")
	}

	seen := map[string]string{"clean": clean}
	record := func(name string) {
		t.Helper()
		value := snapshot()
		for previous, fingerprint := range seen {
			if fingerprint == value {
				t.Fatalf("%s snapshot equals %s snapshot; the change was not fingerprinted", name, previous)
			}
		}
		seen[name] = value
	}

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record("unstaged edit")
	reseedInventoryGit(t, root, "add", "tracked.txt")
	record("staged edit")
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	record("untracked file")
	if err := os.WriteFile(filepath.Join(root, "new.txt"), []byte("b"), 0o600); err != nil {
		t.Fatal(err)
	}
	record("untracked content")
	if err := os.Chmod(filepath.Join(root, "new.txt"), 0o700); err != nil {
		t.Fatal(err)
	}
	record("untracked mode")
	reseedInventoryGit(t, root, "commit", "--message", "second")
	record("new head")
}

func TestReseedRequireCanonicalWorktreeRejectsNonCanonicalRoots(t *testing.T) {
	root := newReseedGitRepo(t, "main")
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: root, Branch: "main"}); err != nil {
		t.Fatalf("canonical worktree rejected: %v", err)
	}
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: root, Branch: "other"}); err == nil || !strings.Contains(err.Error(), "branch mismatch") {
		t.Fatalf("branch mismatch err=%v", err)
	}
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: filepath.Join(root, "absent"), Branch: "main"}); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("missing root err=%v", err)
	}
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: filepath.Join(root, "tracked.txt"), Branch: "main"}); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("file root err=%v", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: link, Branch: "main"}); err == nil || !strings.Contains(err.Error(), "real directory") {
		t.Fatalf("symlink root err=%v", err)
	}
	subdir := filepath.Join(root, "sub")
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: subdir, Branch: "main"}); err == nil || !strings.Contains(err.Error(), "does not match Git top-level") {
		t.Fatalf("subdirectory err=%v", err)
	}
	plain, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := reseedRequireCanonicalWorktree(leasecontract.Workspace{Root: plain, Branch: "main"}); err == nil || !strings.Contains(err.Error(), "does not match Git top-level") {
		t.Fatalf("non-git dir err=%v", err)
	}
}

func TestReseedWorkspaceSnapshotRejectsUntrackedSymlink(t *testing.T) {
	root := newReseedGitRepo(t, "links")
	if err := os.Symlink("tracked.txt", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := reseedWorkspaceSnapshot(leasecontract.Workspace{Root: root, Branch: "links"}); err == nil || !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("untracked symlink err=%v", err)
	}
}

func TestReseedGitOutputReportsFailureWithCommandAndStderr(t *testing.T) {
	root := newReseedGitRepo(t, "main")
	out, err := reseedGitOutput(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil || out != "main" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if _, err := reseedGitOutput(root, "rev-parse", "--verify", "refs/heads/does-not-exist"); err == nil || !strings.Contains(err.Error(), "git rev-parse --verify refs/heads/does-not-exist") {
		t.Fatalf("failure err=%v", err)
	}
	code, _, stderr := reseedGitRaw(filepath.Join(root, "missing-dir"), "status")
	if code == 0 || stderr == "" {
		t.Fatalf("missing dir code=%d stderr=%q", code, stderr)
	}
}

func orcaReseedRecord(root string) leasecontract.Record {
	return leasecontract.Record{
		ID: "io-reseed-guards",
		Execution: &leasecontract.Execution{
			Mode:      "direct",
			Workspace: leasecontract.Workspace{SourceRoot: root, Root: root, Branch: "guards", Driver: "git"},
			Lease:     leasecontract.Lease{Generation: 2, Status: "claimable"},
		},
	}
}

func TestReseedInventoryObserveRejectsIncompleteWiring(t *testing.T) {
	root := newReseedGitRepo(t, "guards")
	inspect := func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "none", leasedomain.ProcessReceipt{}, nil
	}
	actor := leasedomain.Actor{Host: "codex", SessionID: "s"}
	record := orcaReseedRecord(root)

	var nilInventory *ReseedInventory
	if _, err := nilInventory.Observe(t.Context(), record, actor); err == nil || !strings.Contains(err.Error(), "requires an execution and process inspector") {
		t.Fatalf("nil inventory err=%v", err)
	}
	if _, err := NewReseedInventory(nil, nil).Observe(t.Context(), record, actor); err == nil {
		t.Fatal("missing process inspector must be rejected")
	}
	if _, err := NewReseedInventory(nil, inspect).Observe(t.Context(), leasecontract.Record{}, actor); err == nil {
		t.Fatal("record without execution must be rejected")
	}

	orca := orcaReseedRecord(root)
	orca.Execution.Mode = "orca"
	if _, err := NewReseedInventory(&reseedInventoryOwnerStub{}, inspect).Observe(t.Context(), orca, actor); err == nil || !strings.Contains(err.Error(), "exact owner terminal and task inventory") {
		t.Fatalf("orca without binding err=%v", err)
	}
	orca.Execution.Orca = &leasecontract.OrcaBinding{RuntimeID: "rt"}
	if _, err := NewReseedInventory(nil, inspect).Observe(t.Context(), orca, actor); err == nil || !strings.Contains(err.Error(), "exact owner terminal and task inventory") {
		t.Fatalf("orca without owner inspector err=%v", err)
	}

	moved := orcaReseedRecord(root)
	moved.Execution.Workspace.Branch = "different"
	if _, err := NewReseedInventory(nil, inspect).Observe(t.Context(), moved, actor); err == nil || !strings.Contains(err.Error(), "branch mismatch") {
		t.Fatalf("branch drift err=%v", err)
	}
}

type failingOwnerInspector struct{ err error }

func (s failingOwnerInspector) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return port.ExecutionOrcaOwnerInventory{}, s.err
}

type recordingOwnerInspector struct {
	request port.ExecutionOrcaOwnerInventoryRequest
}

func (s *recordingOwnerInspector) InspectOwner(_ context.Context, request port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	s.request = request
	return port.ExecutionOrcaOwnerInventory{RuntimeID: " rt-live "}, nil
}

func TestReseedInventoryObservePropagatesInspectorErrorsAndBindsHolderProcess(t *testing.T) {
	root := newReseedGitRepo(t, "guards")
	actor := leasedomain.Actor{Host: "codex", SessionID: "s", Process: &leasedomain.ProcessReceipt{PID: 9, StartedAt: "2026-01-01T00:00:00Z", Executable: "/bin/codex"}}
	holder := &leasecontract.Actor{Host: "codex", SessionID: "holder", SessionProcess: &leasecontract.ProcessReceipt{PID: 77, StartedAt: "2026-01-01T00:00:00Z", Executable: "/bin/codex"}}

	held := orcaReseedRecord(root)
	held.Execution.Lease = leasecontract.Lease{Generation: 2, Status: "active", Holder: holder}

	probeErr := errors.New("ps failed")
	var probed leasedomain.ProcessReceipt
	if _, err := NewReseedInventory(nil, func(_ context.Context, receipt leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		probed = receipt
		return "", leasedomain.ProcessReceipt{}, probeErr
	}).Observe(t.Context(), held, actor); !errors.Is(err, probeErr) || probed.PID != 77 {
		t.Fatalf("process probe err=%v probed=%+v", err, probed)
	}

	status := "live"
	inspect := func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return status, leasedomain.ProcessReceipt{}, nil
	}
	live, err := NewReseedInventory(nil, inspect).Observe(t.Context(), held, actor)
	if err != nil {
		t.Fatal(err)
	}
	status = "dead"
	dead, err := NewReseedInventory(nil, inspect).Observe(t.Context(), held, actor)
	if err != nil {
		t.Fatal(err)
	}
	if live.Fingerprint == dead.Fingerprint {
		t.Fatal("holder process liveness must be part of the reseed fingerprint")
	}
	otherRequester := actor
	otherRequester.SessionID = "someone-else"
	changed, err := NewReseedInventory(nil, inspect).Observe(t.Context(), held, otherRequester)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Fingerprint == dead.Fingerprint {
		t.Fatal("requester identity must be part of the reseed fingerprint")
	}

	orca := orcaReseedRecord(root)
	orca.Execution.Mode = "orca"
	orca.Execution.Orca = &leasecontract.OrcaBinding{RuntimeID: "rt-old", WorktreeID: "wt", RunID: "run", TaskID: "task", DispatchID: "d", TerminalPTYID: "pty"}
	ownerErr := errors.New("owner unreachable")
	if _, err := NewReseedInventory(failingOwnerInspector{ownerErr}, inspect).Observe(t.Context(), orca, actor); !errors.Is(err, ownerErr) {
		t.Fatalf("owner err=%v", err)
	}

	owner := &recordingOwnerInspector{}
	receipt, err := NewReseedInventory(owner, inspect).Observe(t.Context(), orca, actor)
	if err != nil {
		t.Fatal(err)
	}
	if !owner.request.AllowRuntimeRollover || owner.request.RuntimeID != "rt-old" || owner.request.TerminalPTYID != "pty" || receipt.RuntimeID != "rt-live" {
		t.Fatalf("request=%+v receipt=%+v", owner.request, receipt)
	}
	orca.Execution.Lease = leasecontract.Lease{Generation: 2, Status: "active", Holder: holder}
	if _, err := NewReseedInventory(owner, inspect).Observe(t.Context(), orca, actor); err != nil {
		t.Fatal(err)
	}
	if owner.request.AllowRuntimeRollover {
		t.Fatal("runtime rollover must not be allowed while a holder owns the lease")
	}
}

func TestReseedInventoryObserveAllowsRuntimeRolloverOnlyForHolderlessReleasedOrClaimableLeases(t *testing.T) {
	root := newReseedGitRepo(t, "guards")
	actor := leasedomain.Actor{Host: "codex", SessionID: "s", Process: &leasedomain.ProcessReceipt{PID: 9, StartedAt: "2026-01-01T00:00:00Z", Executable: "/bin/codex"}}
	inspect := func(context.Context, leasedomain.ProcessReceipt) (string, leasedomain.ProcessReceipt, error) {
		return "live", leasedomain.ProcessReceipt{}, nil
	}
	for status, want := range map[string]bool{"released": true, "claimable": true, "active": false} {
		t.Run(status, func(t *testing.T) {
			record := orcaReseedRecord(root)
			record.Execution.Mode = "orca"
			record.Execution.Orca = &leasecontract.OrcaBinding{RuntimeID: "rt-old", WorktreeID: "wt", RunID: "run", TaskID: "task", DispatchID: "d", TerminalPTYID: "pty"}
			record.Execution.Lease = leasecontract.Lease{Generation: 2, Status: status}
			owner := &recordingOwnerInspector{}
			if _, err := NewReseedInventory(owner, inspect).Observe(t.Context(), record, actor); err != nil {
				t.Fatal(err)
			}
			if owner.request.AllowRuntimeRollover != want {
				t.Fatalf("status %q AllowRuntimeRollover=%t want %t", status, owner.request.AllowRuntimeRollover, want)
			}
		})
	}
}

func TestReseedIndexBytesReportsGitFailureAndUnreadableIndex(t *testing.T) {
	if _, err := reseedIndexBytes(filepath.Join(t.TempDir(), "missing-dir")); err == nil || !strings.Contains(err.Error(), "git rev-parse --git-path index") {
		t.Fatalf("git failure err=%v", err)
	}

	root := newReseedGitRepo(t, "index")
	index := filepath.Join(root, ".git", "index")
	if data, err := reseedIndexBytes(root); err != nil || len(data) == 0 {
		t.Fatalf("staged repo index len=%d err=%v", len(data), err)
	}
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if data, err := reseedIndexBytes(root); err != nil || len(data) != 0 {
		t.Fatalf("absent index must read as empty: len=%d err=%v", len(data), err)
	}
	if err := os.Mkdir(index, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := reseedIndexBytes(root); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("unreadable index must fail closed, err=%v", err)
	}
}
