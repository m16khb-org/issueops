package issueops

import (
	"context"
	"fmt"
	"strings"

	cleanupapp "issueops/internal/application/issueopscleanup"
	"issueops/internal/contract/issueops"
)

// CleanupAbandonRuntime provides the filesystem, git and process observations that cleanup
// abandon needs. A nil Git falls back to the default sync-base git runner, and Processes
// overrides process observation and termination.
type CleanupAbandonRuntime struct {
	StateRoot string
	Git       func(string, ...string) (int, string)
	Processes CleanupProcessDeps
}

// Directory reports whether path is an existing directory; a non-directory is an error.
func (CleanupAbandonRuntime) Directory(path string) (bool, error) {
	return (CleanupFinishEnvironment{}).Directory(path)
}

// SameLinkedPath reports whether a and b are the same path after cleaning to absolute form.
func (CleanupAbandonRuntime) SameLinkedPath(a, b string) bool {
	return (CleanupFinishEnvironment{}).SamePath(a, b)
}

// SamePath reports whether a and b name the same location, resolving symlinks in the
// existing part of each path.
func (CleanupAbandonRuntime) SamePath(a, b string) bool { return samePath(a, b) }

// ReadChild reads the existing child cycle record id without creating state.
func (r CleanupAbandonRuntime) ReadChild(id string) (issueops.Record, error) {
	return ReadIssueOpsExisting(r.StateRoot, id)
}

// Workspace observes the worktree at root for the record and returns the observation plus
// its residual notes.
func (r CleanupAbandonRuntime) Workspace(ctx context.Context, record issueops.Record, root string) (cleanupapp.FinishWorkspaceObservation, []string) {
	return newCleanupWorkspaceCleaner(r.Processes).Observe(root, record.Repo)
}

// Worktree observes the worktree at root: whether it is the canonical top-level, its
// branch, HEAD, and whether it is clean. Facts git cannot report stay zero-valued.
func (r CleanupAbandonRuntime) Worktree(ctx context.Context, root string) cleanupapp.AbandonWorktreeObservation {
	var observed cleanupapp.AbandonWorktreeObservation
	if code, out := r.Command(ctx, root, "rev-parse", "--show-toplevel"); code == 0 {
		observed.Canonical = samePath(out, root)
	}
	if code, out := r.Command(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD"); code == 0 {
		observed.Branch = strings.TrimSpace(out)
	}
	code, out := r.Command(ctx, root, "rev-parse", "HEAD")
	observed.HeadObservable = code == 0
	if observed.HeadObservable {
		observed.Head = strings.TrimSpace(out)
	}
	if code, out := r.Command(ctx, root, "status", "--porcelain=v1"); code == 0 {
		observed.Clean = strings.TrimSpace(out) == ""
	}
	return observed
}

// BranchOID returns the OID of the local branch in repo, or an empty string when the
// branch does not exist. Any other git failure is an error.
func (r CleanupAbandonRuntime) BranchOID(ctx context.Context, repo, branch string) (string, error) {
	code, out := r.Command(ctx, repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if code == 1 {
		return "", nil
	}
	if code != 0 || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("local branch readback failed (git exit %d)", code)
	}
	return strings.TrimSpace(out), nil
}

// BranchCheckoutPath returns the path of the worktree that has branch checked out, or an
// empty string when none does.
func (r CleanupAbandonRuntime) BranchCheckoutPath(ctx context.Context, repo, branch string) (string, error) {
	code, out := r.Command(ctx, repo, "worktree", "list", "--porcelain")
	if code != 0 {
		return "", fmt.Errorf("worktree registry readback failed (git exit %d)", code)
	}
	return cleanupAbandonBranchCheckoutPath(out, branch), nil
}

func cleanupAbandonBranchCheckoutPath(output, branch string) string {
	currentPath := ""
	for line := range strings.SplitSeq(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.TrimSpace(line) == "branch refs/heads/"+branch:
			return currentPath
		}
	}
	return ""
}

// Command uses the inherited execution context for all default Git processes.
func (r CleanupAbandonRuntime) Command(ctx context.Context, dir string, args ...string) (int, string) {
	if r.Git != nil {
		return r.Git(dir, args...)
	}
	return defaultExecutionSyncBaseGit(ctx, dir, args...)
}

// Stop terminates the workspace processes listed for the inventory worktree and returns
// those that remain.
func (r CleanupAbandonRuntime) Stop(ctx context.Context, inventory issueops.CleanupAbandonInventory, processes []issueops.CleanupWorkspaceProcess) ([]issueops.CleanupWorkspaceProcess, error) {
	return newCleanupWorkspaceCleaner(r.Processes).Stop(inventory.WorktreeRoot, processes)
}
