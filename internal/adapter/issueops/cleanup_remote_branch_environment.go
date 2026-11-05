package issueops

import (
	"context"
	"fmt"
	"strings"
)

// CleanupRemoteBranchEnvironment performs the git steps of remote-branch cleanup. A nil
// RunGit falls back to the default sync-base git runner.
type CleanupRemoteBranchEnvironment struct {
	RunGit func(context.Context, string, ...string) (int, string)
}

func (s CleanupRemoteBranchEnvironment) git(ctx context.Context, repo string, args ...string) (int, string) {
	if s.RunGit != nil {
		return s.RunGit(ctx, repo, args...)
	}
	return defaultExecutionSyncBaseGit(ctx, repo, args...)
}

// RemoteRef observes the OID of branch on the origin of repo.
func (s CleanupRemoteBranchEnvironment) RemoteRef(ctx context.Context, repo, branch string) (string, error) {
	return (LinkedBranchRemoteRef{RunGit: s.git}).Observe(ctx, repo, branch)
}

// OriginURL returns the trimmed origin remote URL of repo or the git failure output.
func (s CleanupRemoteBranchEnvironment) OriginURL(ctx context.Context, repo string) (string, error) {
	code, out := s.git(ctx, repo, "remote", "get-url", "origin")
	if code != 0 {
		return "", fmt.Errorf("git remote get-url origin: %s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}

// TipReachedBase reports whether oid is already an ancestor of the remote-tracking base
// branch, i.e. the branch tip has been merged.
func (s CleanupRemoteBranchEnvironment) TipReachedBase(ctx context.Context, repo, oid, base string) bool {
	code, _ := s.git(ctx, repo, "merge-base", "--is-ancestor", oid, "refs/remotes/origin/"+base)
	return code == 0
}

// Delete removes branch from origin with a lease on oid, so a branch that moved since
// the preview is left untouched.
func (s CleanupRemoteBranchEnvironment) Delete(ctx context.Context, repo, branch, oid string) error {
	ref := "refs/heads/" + branch
	code, out := s.git(ctx, repo, "push", "origin", "--delete", ref, "--force-with-lease="+ref+":"+oid)
	if code != 0 {
		return fmt.Errorf("git push origin --delete refs/heads/%s failed (remote unchanged; re-run preview then apply): %s", branch, strings.TrimSpace(out))
	}
	return nil
}
