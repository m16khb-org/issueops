package gitworktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/preflight"
	"issueops/internal/port"
)

// Prepare의 실제 계약을 잠근다: dry-run은 생성하지 않고, confirm은 형제
// .worktrees base에 실제 worktree를 만들며, 이미 있으면 identity가 일치할 때
// 통과하고 불일치하면 fail-closed로 거부한다. 정확히 dogfooding에서 밟았던
// 경로다(worktree 디렉터리 이름은 브랜치명과 일치해야 prepare가 만들 수 있다).
func TestPrepareCreatesCanonicalWorktreeOnConfirm(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-69", SourceRoot: repo,
		Root:   filepath.Join(base, "69-prepare"),
		Branch: "69-prepare", BaseBranch: "main",
		BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"),
		Confirm:  true,
	}
	if _, err := testProvisioner().Prepare(context.Background(), req); err != nil {
		t.Fatalf("prepare confirm: %v", err)
	}
	if info, err := os.Lstat(req.Root); err != nil || !info.IsDir() {
		t.Fatalf("prepared worktree missing: %v", err)
	}
	if got := preflight.GitOut(req.Root, "branch", "--show-current"); got != req.Branch {
		t.Fatalf("prepared branch = %q want %q", got, req.Branch)
	}
	// 동일 요청 재실행은 기존 worktree identity로 통과해야 한다(멱등).
	receipt, err := testProvisioner().Prepare(context.Background(), req)
	if err != nil || !receipt.Exists {
		t.Fatalf("re-prepare must be idempotent: receipt=%#v err=%v", receipt, err)
	}
}

func TestPrepareDryRunDoesNotCreateWorktree(t *testing.T) {
	repo := initAccessRepo(t)
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-70", SourceRoot: repo,
		Root:   repo + ".worktrees/70-dry",
		Branch: "70-dry", BaseBranch: "main",
		BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"),
	}
	receipt, err := testProvisioner().Prepare(context.Background(), req)
	if err != nil {
		t.Fatalf("dry-run prepare: %v", err)
	}
	if receipt.Exists {
		t.Fatalf("dry-run must not report an existing worktree: %#v", receipt)
	}
	if _, err := os.Lstat(req.Root); !os.IsNotExist(err) {
		t.Fatalf("dry-run created a worktree: %v", err)
	}
}

func TestPrepareRejectsInvalidRequests(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	cases := []struct {
		name string
		req  port.ExecutionWorkspaceRequest
		want string
	}{
		{
			name: "missing identity fields",
			req:  port.ExecutionWorkspaceRequest{SourceRoot: repo, Root: filepath.Join(base, "x"), Branch: "x", BaseHead: "a"},
			want: "lifecycle_id, source_root, root, branch, and base_head are required",
		},
		{
			name: "non-sibling base",
			req: port.ExecutionWorkspaceRequest{
				LifecycleID: "io-71", SourceRoot: repo, Root: filepath.Join(filepath.Dir(repo), "elsewhere", "71-x"),
				Branch: "71-x", BaseBranch: "main", BaseHead: "a",
			},
			want: "sibling .worktrees base",
		},
		{
			// 검증 순서 계약: sibling-base 검사가 isolation 검사보다 먼저다.
			// root==source는 base 위반이기도 하므로 base 에러가 먼저 나온다.
			name: "root equals source",
			req: port.ExecutionWorkspaceRequest{
				LifecycleID: "io-72", SourceRoot: repo, Root: repo,
				Branch: "72-x", BaseBranch: "main", BaseHead: "a",
			},
			want: "sibling .worktrees base",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := testProvisioner().Prepare(context.Background(), tc.req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestPrepareRejectsOccupiedNonWorktreePath(t *testing.T) {
	repo := initAccessRepo(t)
	root := filepath.Join(repo+".worktrees", "73-occupied")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	// 일반 디렉터리(=git worktree가 아님)면 identity 검증이 실패해야 한다.
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-73", SourceRoot: repo, Root: root,
		Branch: "73-occupied", BaseBranch: "main",
		BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"),
		Confirm:  true,
	}
	if _, err := testProvisioner().Prepare(context.Background(), req); err == nil {
		t.Fatal("occupied non-worktree path must fail closed")
	}
}

// 9단계 흐름의 핵심 기제: 2단계가 base SHA에 미리 만든 워크트리를 3단계의
// prepare가 채택한다. 채택이 깨지면 두 번째 checkout이 생기거나 사이클이
// 시작되지 못하므로, 여기서 성공·실패 양쪽을 고정한다(T0b 실측 1).
func TestPrepareAdoptsPreCreatedWorktreeAtBaseSHA(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "74-adopt")
	head := preflight.GitOut(repo, "rev-parse", "HEAD")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", root, "-b", "74-adopt", head); code != 0 {
		t.Fatalf("pre-create worktree: %s", stderr)
	}
	before := worktreeCount(t, repo)

	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-74", SourceRoot: repo, Root: root,
		Branch: "74-adopt", BaseBranch: "main", BaseHead: head, Confirm: true,
	}
	receipt, err := testProvisioner().Prepare(context.Background(), req)
	if err != nil {
		t.Fatalf("adopting a pre-created worktree must succeed: %v", err)
	}
	if !receipt.Exists {
		t.Fatalf("adoption must report an existing worktree: %#v", receipt)
	}
	if after := worktreeCount(t, repo); after != before {
		t.Fatalf("adoption created a second checkout: before=%d after=%d", before, after)
	}
	if got := preflight.GitOut(root, "rev-parse", "HEAD"); got != head {
		t.Fatalf("adopted worktree HEAD = %q want %q", got, head)
	}
}

func TestPrepareRejectsAdoptedWorktreeThatMovedPastBaseSHA(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "75-moved")
	head := preflight.GitOut(repo, "rev-parse", "HEAD")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", root, "-b", "75-moved", head); code != 0 {
		t.Fatalf("pre-create worktree: %s", stderr)
	}
	if err := os.WriteFile(filepath.Join(root, "extra.txt"), []byte("one commit past base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "extra.txt"}, {"commit", "-q", "-m", "past base"}} {
		if code, _, stderr := preflight.GitCmd(root, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	before := worktreeCount(t, repo)

	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-75", SourceRoot: repo, Root: root,
		Branch: "75-moved", BaseBranch: "main", BaseHead: head, Confirm: true,
	}
	_, err := testProvisioner().Prepare(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "does not match branch and base_head") {
		t.Fatalf("a worktree past the base SHA must fail closed, got %v", err)
	}
	if after := worktreeCount(t, repo); after != before {
		t.Fatalf("a rejected adoption must not change the worktree inventory: before=%d after=%d", before, after)
	}
}

func TestPrepareRejectsAdoptedWorktreeOnAnotherBranch(t *testing.T) {
	repo := initAccessRepo(t)
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "76-branch")
	head := preflight.GitOut(repo, "rev-parse", "HEAD")
	if code, _, stderr := preflight.GitCmd(repo, "worktree", "add", "-q", root, "-b", "76-other", head); code != 0 {
		t.Fatalf("pre-create worktree: %s", stderr)
	}
	req := port.ExecutionWorkspaceRequest{
		LifecycleID: "io-76", SourceRoot: repo, Root: root,
		Branch: "76-branch", BaseBranch: "main", BaseHead: head, Confirm: true,
	}
	if _, err := testProvisioner().Prepare(context.Background(), req); err == nil ||
		!strings.Contains(err.Error(), "does not match branch and base_head") {
		t.Fatalf("a worktree on another branch must fail closed, got %v", err)
	}
}

func worktreeCount(t *testing.T, repo string) int {
	t.Helper()
	out := preflight.GitOut(repo, "worktree", "list", "--porcelain")
	return strings.Count(out, "worktree ")
}

// A branch that already moved past the sealed base is refused, and the
// worktree this call added is removed so a retry is not refused by a
// leftover mismatched worktree. The pre-existing branch itself is kept.
func TestPrepareRemovesTheWorktreeItAddedWhenTheBranchIsPastTheBase(t *testing.T) {
	repo := initAccessRepo(t)
	baseHead := preflight.GitOut(repo, "rev-parse", "HEAD")
	for _, args := range [][]string{{"branch", "70-ahead"}, {"commit", "-q", "--allow-empty", "-m", "ahead"}} {
		if args[0] == "commit" {
			if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "70-ahead"); code != 0 {
				t.Fatal(stderr)
			}
		}
		if code, _, stderr := preflight.GitCmd(repo, args...); code != 0 {
			t.Fatalf("git %v: %s", args, stderr)
		}
	}
	if code, _, stderr := preflight.GitCmd(repo, "checkout", "-q", "main"); code != 0 {
		t.Fatal(stderr)
	}
	base := repo + ".worktrees"
	if err := os.Mkdir(base, 0o700); err != nil {
		t.Fatal(err)
	}
	req := port.ExecutionWorkspaceRequest{LifecycleID: "io-70", SourceRoot: repo, Root: filepath.Join(base, "70-ahead"), Branch: "70-ahead", BaseBranch: "main", BaseHead: baseHead, Confirm: true}
	_, err := testProvisioner().Prepare(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "sealed base_head is "+baseHead[:10]) {
		t.Fatalf("prepare = %v", err)
	}
	if _, statErr := os.Lstat(req.Root); !os.IsNotExist(statErr) {
		t.Fatalf("refused prepare left the worktree: %v", statErr)
	}
	if preflight.GitOut(repo, "rev-parse", "--verify", "-q", "refs/heads/70-ahead") == "" {
		t.Fatal("pre-existing branch was deleted")
	}
}

// Probe: a leftover plain directory at the canonical path refused with the
// branch/base_head message; the refusal names the directory instead.
func TestPrepareNamesAPlainDirectoryAtTheCanonicalPath(t *testing.T) {
	repo := initAccessRepo(t)
	root := filepath.Join(repo+".worktrees", "71-occupied")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	req := port.ExecutionWorkspaceRequest{LifecycleID: "io-71", SourceRoot: repo, Root: root, Branch: "71-occupied", BaseBranch: "main", BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"), Confirm: true}
	_, err := testProvisioner().Prepare(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "is not a git worktree of its own") {
		t.Fatalf("prepare = %v", err)
	}
}

// Probe: the source checkout was on the cycle branch; the refusal names the
// switch that frees it.
func TestPrepareNamesABranchCheckedOutInTheSourceCheckout(t *testing.T) {
	repo := initAccessRepo(t)
	if code, _, stderr := preflight.GitCmd(repo, "switch", "-q", "-c", "72-busy"); code != 0 {
		t.Fatal(stderr)
	}
	req := port.ExecutionWorkspaceRequest{LifecycleID: "io-72", SourceRoot: repo, Root: filepath.Join(repo+".worktrees", "72-busy"), Branch: "72-busy", BaseBranch: "main", BaseHead: preflight.GitOut(repo, "rev-parse", "HEAD"), Confirm: true}
	if err := os.MkdirAll(repo+".worktrees", 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := testProvisioner().Prepare(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "is checked out elsewhere") {
		t.Fatalf("prepare = %v", err)
	}
}
