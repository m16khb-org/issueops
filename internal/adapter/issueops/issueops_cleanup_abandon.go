package issueops

import (
	"context"
	"fmt"
	"strings"

	cleanupapp "issueops/internal/application/issueopscleanup"
	"issueops/internal/contract/issueops"
	preparation "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

type CleanupAbandonRuntime struct {
	StateRoot     string
	Git           func(string, ...string) (int, string)
	Processes     CleanupProcessDeps
	OrcaTerminals port.CleanupOrcaTerminals
}

func (CleanupAbandonRuntime) Directory(path string) (bool, error) {
	return (CleanupFinishEnvironment{}).Directory(path)
}
func (CleanupAbandonRuntime) SameLinkedPath(a, b string) bool {
	return (CleanupFinishEnvironment{}).SamePath(a, b)
}
func (CleanupAbandonRuntime) SamePath(a, b string) bool { return samePath(a, b) }
func (r CleanupAbandonRuntime) ReadChild(id string) (issueops.IssueOpsRecord, error) {
	return ReadIssueOpsExisting(r.StateRoot, id)
}
func (r CleanupAbandonRuntime) ReadIntent(id string) (preparation.Intent, error) {
	return ReadExecutionOrcaIntent(r.StateRoot, id)
}
func (r CleanupAbandonRuntime) Workspace(ctx context.Context, record issueops.IssueOpsRecord, root string) (cleanupapp.FinishWorkspaceObservation, []string) {
	return NewCleanupWorkspaceCleaner(r.Processes, r.OrcaTerminals).Observe(ctx, record, root)
}
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
func (r CleanupAbandonRuntime) BranchCheckoutPath(ctx context.Context, repo, branch string) (string, error) {
	code, out := r.Command(ctx, repo, "worktree", "list", "--porcelain")
	if code != 0 {
		return "", fmt.Errorf("worktree registry readback failed (git exit %d)", code)
	}
	return cleanupAbandonBranchCheckoutPath(out, branch), nil
}

func cleanupAbandonBranchCheckoutPath(output, branch string) string {
	currentPath := ""
	for _, line := range strings.Split(output, "\n") {
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

func (r CleanupAbandonRuntime) Stop(ctx context.Context, inventory issueops.CleanupAbandonInventory, processes []issueops.CleanupWorkspaceProcess) ([]issueops.CleanupWorkspaceProcess, int, error) {
	return NewCleanupWorkspaceCleaner(r.Processes, r.OrcaTerminals).Stop(ctx, inventory.WorktreeRoot, processes, inventory.OrcaTerminals, inventory.OrcaRuntimeReady, inventory.OrcaAppPID)
}
