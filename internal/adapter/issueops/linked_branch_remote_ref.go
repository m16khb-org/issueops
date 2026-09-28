package issueops

import (
	"context"
	"fmt"
	"strings"
)

type LinkedBranchRemoteRef struct {
	RunGit func(context.Context, string, ...string) (int, string)
}

// Observe distinguishes a successful empty advertisement from transport or
// malformed output. Only a successful empty result proves the ref is absent.
func (s LinkedBranchRemoteRef) Observe(ctx context.Context, repo, branch string) (string, error) {
	run := s.RunGit
	if run == nil {
		run = defaultExecutionSyncBaseGit
	}
	ref := "refs/heads/" + branch
	code, out := run(ctx, repo, "ls-remote", "--heads", "origin", ref)
	if code != 0 {
		return "", fmt.Errorf("linked branch remote ref readback failed (git exit %d)", code)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", nil
	}
	fields := strings.Fields(out)
	if len(fields) != 2 || fields[1] != ref {
		return "", fmt.Errorf("linked branch remote ref readback is malformed")
	}
	return fields[0], nil
}
