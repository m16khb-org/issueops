package gitworktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

func gitMust(t *testing.T, dir string, args ...string) {
	t.Helper()
	if code, _, stderr := preflight.GitCmd(dir, args...); code != 0 {
		t.Fatalf("git %v: %s", args, stderr)
	}
}

func newRequest(t *testing.T, repo, branch string) port.ExecutionWorkspaceRequest {
	t.Helper()
	return port.ExecutionWorkspaceRequest{
		LifecycleID: "io-" + branch, SourceRoot: repo, Root: filepath.Join(repo+".worktrees", branch),
		Branch: branch, BaseBranch: "main", BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"), Confirm: true,
	}
}

func TestPrepareAndProbeAccessFailFastOnCancelledContext(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "80-cancel")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := testProvisioner().Prepare(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("Prepare error = %v, want context.Canceled", err)
	}
	if _, err := testProvisioner().ProbeAccess(ctx, req, "codex"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ProbeAccess error = %v, want context.Canceled", err)
	}
	if _, err := os.Lstat(repo + ".worktrees"); !os.IsNotExist(err) {
		t.Fatalf("cancelled calls must not touch the filesystem: %v", err)
	}
}

func TestPrepareRejectsSourceRootThatIsNotARealGitTopLevel(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	sub := filepath.Join(repo, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "repo-link")
	if err := os.Symlink(repo, link); err != nil {
		t.Fatal(err)
	}
	plainFile := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(plainFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	head := preflight.GitOut(repo, "rev-parse", "HEAD")

	for _, tc := range []struct{ name, source, want string }{
		{"symlinked source", link, "source_root must be a real directory"},
		{"missing source", filepath.Join(t.TempDir(), "absent"), "source_root must be a real directory"},
		{"file source", plainFile, "source_root must be a real directory"},
		{"subdirectory of a repo", sub, "source_root must be the Git top-level"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := port.ExecutionWorkspaceRequest{
				LifecycleID: "io-81", SourceRoot: tc.source, Root: filepath.Join(tc.source+".worktrees", "81-x"),
				Branch: "81-x", BaseBranch: "main", BaseHead: head, Confirm: true,
			}
			_, err := testProvisioner().Prepare(t.Context(), req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Prepare error = %v, want %q", err, tc.want)
			}
			if _, probeErr := testProvisioner().ProbeAccess(t.Context(), req, "codex"); probeErr == nil || !strings.Contains(probeErr.Error(), tc.want) {
				t.Fatalf("ProbeAccess error = %v, want %q", probeErr, tc.want)
			}
		})
	}
	if _, err := os.Lstat(sub + ".worktrees"); !os.IsNotExist(err) {
		t.Fatalf("rejected request must not create a base: %v", err)
	}
}

func TestPrepareRejectsNonRepositorySourceRootEvenWhenItIsTheWorkingDirectory(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	parent := t.TempDir()
	plain := filepath.Join(parent, "plain")
	if err := os.Mkdir(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", parent)
	// git rev-parse fails here and yields an empty top-level; filepath.Abs("") is the cwd,
	// so the check must not accept the source root just because it is the working directory.
	t.Chdir(plain)
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-82", SourceRoot: plain, Root: filepath.Join(plain+".worktrees", "82-x"),
		Branch: "82-x", BaseBranch: "main", BaseHead: strings.Repeat("a", 40), Confirm: true,
	}
	if _, err := testProvisioner().Prepare(t.Context(), req); err == nil || !strings.Contains(err.Error(), "source_root must be the Git top-level") {
		t.Fatalf("Prepare error = %v, want Git top-level rejection", err)
	}
	if _, err := os.Lstat(plain + ".worktrees"); !os.IsNotExist(err) {
		t.Fatalf("rejected request must not create a base: %v", err)
	}
}

func TestPrepareRejectsSymlinkAtCanonicalRoot(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "82-link")
	if err := os.Mkdir(repo+".worktrees", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), req.Root); err != nil {
		t.Fatal(err)
	}
	_, err := testProvisioner().Prepare(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), "occupied by a non-directory or symlink") {
		t.Fatalf("error = %v", err)
	}
}

func TestPrepareRejectsBaseThatIsNotARealDirectory(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	for name, setup := range map[string]func(t *testing.T, base string){
		"file": func(t *testing.T, base string) {
			if err := os.WriteFile(base, []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"symlink": func(t *testing.T, base string) {
			if err := os.Symlink(t.TempDir(), base); err != nil {
				t.Fatal(err)
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			repo := initAccessRepo(t)
			req := newRequest(t, repo, "83-base")
			setup(t, repo+".worktrees")
			// A file at the base makes Lstat(root) fail with ENOTDIR (occupied
			// error); a symlinked base resolves, so the base check is the one
			// that rejects. Either way nothing may be created.
			_, err := testProvisioner().Prepare(t.Context(), req)
			if err == nil {
				t.Fatal("non-directory base must be rejected")
			}
			if name == "symlink" && !strings.Contains(err.Error(), "canonical worktree base must be a real directory") {
				t.Fatalf("symlink base error = %v", err)
			}
			if worktreeCount(t, repo) != 1 {
				t.Fatalf("rejected prepare must not register a worktree")
			}
		})
	}
}

func TestPrepareCreatesMissingBaseWithOwnerOnlyMode(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "84-mkbase")
	receipt, err := testProvisioner().Prepare(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Exists || receipt.Branch != req.Branch {
		t.Fatalf("receipt = %#v", receipt)
	}
	info, err := os.Stat(repo + ".worktrees")
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("created base mode = %v, want 0700", info.Mode().Perm())
	}
}

func TestPrepareReportsGitFailureWhenBranchIsCheckedOutElsewhere(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	// "main" is checked out in the source, so adding a second worktree on it fails.
	req := newRequest(t, repo, "main")
	receipt, err := testProvisioner().Prepare(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), "git worktree add") {
		t.Fatalf("error = %v, want git worktree add failure", err)
	}
	if receipt.Exists {
		t.Fatalf("failed add must not report Exists: %#v", receipt)
	}
	if _, statErr := os.Lstat(req.Root); statErr == nil {
		t.Fatal("failed add must not leave a worktree directory")
	}
}

func TestPrepareRejectsFreshWorktreeWhoseBranchIsNotAtBaseHead(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	base := preflight.GitOut(repo, "rev-parse", "HEAD")
	// An existing local branch is checked out as-is, ahead of the requested base.
	gitMust(t, repo, "checkout", "-q", "-b", "85-ahead")
	if err := os.WriteFile(filepath.Join(repo, "more.txt"), []byte("m"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitMust(t, repo, "add", "more.txt")
	gitMust(t, repo, "commit", "-q", "-m", "ahead")
	gitMust(t, repo, "checkout", "-q", "main")

	req := newRequest(t, repo, "85-ahead")
	req.BaseHead = base
	receipt, err := testProvisioner().Prepare(t.Context(), req)
	if err == nil || !strings.Contains(err.Error(), "does not match branch and base_head") {
		t.Fatalf("error = %v, want identity mismatch", err)
	}
	if receipt.Exists {
		t.Fatalf("mismatched worktree must not be reported as ready: %#v", receipt)
	}
}

func TestPrepareStartsBranchFromRemoteTrackingRefWhenNoLocalBranchExists(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	head := preflight.GitOut(repo, "rev-parse", "HEAD")
	gitMust(t, repo, "remote", "add", "origin", filepath.Join(t.TempDir(), "none"))
	gitMust(t, repo, "update-ref", "refs/remotes/origin/86-remote", head)

	req := newRequest(t, repo, "86-remote")
	if _, err := testProvisioner().Prepare(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	if got := preflight.GitOut(req.Root, "branch", "--show-current"); got != "86-remote" {
		t.Fatalf("branch = %q, want a local branch created from the remote ref", got)
	}
	if got := preflight.GitOut(repo, "show-ref", "--verify", "--hash", "refs/heads/86-remote"); got != head {
		t.Fatalf("local branch = %q, want %q", got, head)
	}
	if got := preflight.GitOut(repo, "rev-parse", "--abbrev-ref", "86-remote@{upstream}"); got != "origin/86-remote" {
		t.Fatalf("upstream = %q, want origin/86-remote (proves the remote-tracking candidate was used, not the base_head fallback)", got)
	}
}

func TestProbeAccessAllowsWritableBaseAndLeavesNoProbeFile(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "87-ok")
	base := repo + ".worktrees"

	// Missing base: probing creates it and reports it usable.
	got, err := testProvisioner().ProbeAccess(t.Context(), req, "codex")
	if err != nil || !got.Allowed || got.Code != "" || got.RelaunchCommand != "" {
		t.Fatalf("missing base result = %#v err=%v", got, err)
	}
	// Existing base: same outcome.
	got, err = testProvisioner().ProbeAccess(t.Context(), req, "claude")
	if err != nil || !got.Allowed {
		t.Fatalf("existing base result = %#v err=%v", got, err)
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("probe must clean up its temp file, found %v", entries)
	}
}

func TestProbeAccessDeniesSymlinkedBaseWithoutRemovingIt(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "88-link")
	base := repo + ".worktrees"
	if err := os.Symlink(t.TempDir(), base); err != nil {
		t.Fatal(err)
	}
	got, err := testProvisioner().ProbeAccess(t.Context(), req, "omp")
	if err != nil {
		t.Fatal(err)
	}
	if got.Allowed || got.Code != "canonical_worktree_base_inaccessible" || !strings.HasSuffix(got.RelaunchCommand, " && omp") {
		t.Fatalf("result = %#v", got)
	}
	if info, err := os.Lstat(base); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("a base this probe did not create must be left in place: %v", err)
	}
}

func TestProbeAccessRejectsUnknownHostOnlyWhenAccessIsDenied(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "89-host")

	// Access is fine, so the host name is never consulted.
	if got, err := testProvisioner().ProbeAccess(t.Context(), req, "vim"); err != nil || !got.Allowed {
		t.Fatalf("allowed probe = %#v err=%v", got, err)
	}
	// Access denied: the relaunch command cannot be built for an unknown host.
	if err := os.Remove(repo + ".worktrees"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), repo+".worktrees"); err != nil {
		t.Fatal(err)
	}
	_, err := testProvisioner().ProbeAccess(t.Context(), req, "vim")
	if err == nil || !strings.Contains(err.Error(), "host must be codex, claude, omo, omp, or opencode") {
		t.Fatalf("error = %v", err)
	}
}

func TestProbeAccessDeniesWhenBaseCannotBeCreatedUnderReadOnlyParent(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	if os.Geteuid() == 0 {
		t.Skip("permission denial is not observable as root")
	}
	repo := initAccessRepo(t)
	req := newRequest(t, repo, "90-rollback")
	base := repo + ".worktrees"

	// The parent loses write permission, so creating the base fails: the
	// probe must answer "denied" with a relaunch command, not an error.
	parent := filepath.Dir(repo)
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	got, err := testProvisioner().ProbeAccess(t.Context(), req, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if got.Allowed || got.Code != "canonical_worktree_base_inaccessible" {
		t.Fatalf("result = %#v", got)
	}
	if _, statErr := os.Lstat(base); !os.IsNotExist(statErr) {
		t.Fatalf("no base may be left behind: %v", statErr)
	}
}

func TestCanonicalBaseGuardsRefuseNonCanonicalTargets(t *testing.T) {
	source := filepath.Join(t.TempDir(), "repo")
	other := filepath.Join(t.TempDir(), "other.worktrees")

	if err := createRealDirectories(source, other); err == nil || !strings.Contains(err.Error(), "non-canonical worktree base") {
		t.Fatalf("createRealDirectories error = %v", err)
	}
	if _, err := os.Lstat(other); !os.IsNotExist(err) {
		t.Fatalf("refused target must not be created: %v", err)
	}
	created, err := ensureProbeBase(source, other)
	if err == nil || created || !strings.Contains(err.Error(), "non-canonical worktree base") {
		t.Fatalf("ensureProbeBase = (%v, %v)", created, err)
	}
	if _, err := os.Lstat(other); !os.IsNotExist(err) {
		t.Fatalf("refused probe target must not be created: %v", err)
	}
}

func TestEnsureProbeBaseReportsCreationOnlyForNewDirectories(t *testing.T) {
	source := filepath.Join(t.TempDir(), "repo")
	base := source + ".worktrees"

	created, err := ensureProbeBase(source, base)
	if err != nil || !created {
		t.Fatalf("first call = (%v, %v), want created", created, err)
	}
	created, err = ensureProbeBase(source, base)
	if err != nil || created {
		t.Fatalf("second call = (%v, %v), want existing and not created", created, err)
	}
}

func TestProbeDirectoryReadWriteFailsOnMissingOrUnwritableDirectory(t *testing.T) {
	if err := probeDirectoryReadWrite(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("missing directory must fail the probe")
	}
	if os.Geteuid() == 0 {
		t.Skip("permission denial is not observable as root")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := probeDirectoryReadWrite(dir); err == nil {
		t.Fatal("read-only directory must fail the write half of the probe")
	}
}

func TestSameResolvedPathFollowsSymlinksAndComparesCleanedPaths(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if !sameResolvedPath(link, real) {
		t.Fatal("a symlink and its target are the same path")
	}
	if !sameResolvedPath(real+"/./", real) {
		t.Fatal("trailing dot segments must not matter")
	}
	if sameResolvedPath(real, dir) {
		t.Fatal("different directories must not match")
	}
	// Nonexistent paths fall back to lexical comparison.
	if !sameResolvedPath(filepath.Join(dir, "ghost"), filepath.Join(dir, "x", "..", "ghost")) {
		t.Fatal("unresolvable paths compare lexically")
	}
}
