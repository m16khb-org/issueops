package issueops

import (
	"context"
	"fmt"
	"strings"
)

type CleanupRemoteBranchEnvironment struct {
	RunGit func(context.Context, string, ...string) (int, string)
}

func (s CleanupRemoteBranchEnvironment) git(ctx context.Context, repo string, args ...string) (int, string) {
	if s.RunGit != nil {
		return s.RunGit(ctx, repo, args...)
	}
	return defaultExecutionSyncBaseGit(ctx, repo, args...)
}
func (s CleanupRemoteBranchEnvironment) RemoteRef(ctx context.Context, repo, branch string) (string, error) {
	return (LinkedBranchRemoteRef{RunGit: s.git}).Observe(ctx, repo, branch)
}
func (s CleanupRemoteBranchEnvironment) OriginURL(ctx context.Context, repo string) (string, error) {
	code, out := s.git(ctx, repo, "remote", "get-url", "origin")
	if code != 0 {
		return "", fmt.Errorf("git remote get-url origin: %s", strings.TrimSpace(out))
	}
	return strings.TrimSpace(out), nil
}
func (s CleanupRemoteBranchEnvironment) TipReachedBase(ctx context.Context, repo, oid, base string) bool {
	code, _ := s.git(ctx, repo, "merge-base", "--is-ancestor", oid, "refs/remotes/origin/"+base)
	return code == 0
}
func (s CleanupRemoteBranchEnvironment) Delete(ctx context.Context, repo, branch, oid string) error {
	ref := "refs/heads/" + branch
	code, out := s.git(ctx, repo, "push", "origin", "--delete", ref, "--force-with-lease="+ref+":"+oid)
	if code != 0 {
		return fmt.Errorf("git push origin --delete refs/heads/%s failed (remote unchanged; re-run preview then apply): %s", branch, strings.TrimSpace(out))
	}
	return nil
}
