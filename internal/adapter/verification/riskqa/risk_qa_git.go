package riskqa

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	riskqacontract "issueops/internal/contract/riskqa"
)

func gitScopedPaths(root, baseRef string) ([]string, *riskqacontract.Scope) {
	scope := &riskqacontract.Scope{}
	// Unscoped callers retain working-tree-only behavior, including unborn HEAD.
	if baseRef != "" {
		base, err := riskGit(root, "rev-parse", "--verify", "--end-of-options", baseRef+"^{commit}")
		if err != nil {
			scope.Error = "risk QA base commit unavailable: " + err.Error()
			return nil, scope
		}
		scope.BaseSHA = strings.TrimSpace(string(base))
		head, err := riskGit(root, "rev-parse", "--verify", "HEAD^{commit}")
		if err != nil {
			scope.Error = "risk QA HEAD commit unavailable: " + err.Error()
			return nil, scope
		}
		scope.HeadSHA = strings.TrimSpace(string(head))
	}
	paths := []string{}
	if baseRef != "" {
		// Disabling rename folding includes both deleted sources and added targets.
		diff, err := riskGit(root, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", scope.BaseSHA, scope.HeadSHA, "--")
		if err != nil {
			scope.Error = "risk QA committed range unavailable: " + err.Error()
			return nil, scope
		}
		paths = append(paths, strings.Split(string(diff), "\x00")...)
	}
	status, err := riskGit(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		scope.Error = "git status unavailable: " + err.Error()
		return nil, scope
	}
	entries := strings.Split(string(status), "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' {
			scope.Error = "invalid NUL git status record"
			return nil, scope
		}
		paths = append(paths, entry[3:])
		if strings.ContainsAny(entry[:2], "RC") {
			i++
			if i >= len(entries) || entries[i] == "" {
				scope.Error = "missing NUL git rename source"
				return nil, scope
			}
			paths = append(paths, entries[i])
		}
	}
	if baseRef == "" {
		scope = nil
	}
	return riskqacontract.NormalizePaths(paths), scope
}

func riskGit(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git observation timed out: %w", ctx.Err())
	}
	return out, err
}
