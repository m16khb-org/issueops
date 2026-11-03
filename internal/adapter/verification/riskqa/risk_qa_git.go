package riskqa

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"issueops/internal/adapter/outbound/gitbin"
	riskqacontract "issueops/internal/contract/riskqa"
)

func gitScopedPaths(root, baseRef string) ([]string, *riskqacontract.Scope) {
	scope := &riskqacontract.Scope{}
	paths := []string{}
	// Unscoped callers retain working-tree-only behavior, including unborn HEAD.
	if baseRef != "" {
		if message := resolveRiskScopeCommits(root, baseRef, scope); message != "" {
			scope.Error = message
			return nil, scope
		}
		committed, message := riskCommittedRangePaths(root, scope)
		if message != "" {
			scope.Error = message
			return nil, scope
		}
		paths = append(paths, committed...)
	}
	statusPaths, message := riskStatusPaths(root)
	if message != "" {
		scope.Error = message
		return nil, scope
	}
	paths = append(paths, statusPaths...)
	if baseRef == "" {
		scope = nil
	}
	return riskqacontract.NormalizePaths(paths), scope
}

// resolveRiskScopeCommits records the base and HEAD commit SHAs on scope and
// returns a non-empty error message when either commit cannot be resolved.
func resolveRiskScopeCommits(root, baseRef string, scope *riskqacontract.Scope) string {
	base, err := riskGit(root, "rev-parse", "--verify", "--end-of-options", baseRef+"^{commit}")
	if err != nil {
		return "risk QA base commit unavailable: " + err.Error()
	}
	scope.BaseSHA = strings.TrimSpace(string(base))
	head, err := riskGit(root, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "risk QA HEAD commit unavailable: " + err.Error()
	}
	scope.HeadSHA = strings.TrimSpace(string(head))
	return ""
}

// riskCommittedRangePaths lists the paths changed between the scope commits.
func riskCommittedRangePaths(root string, scope *riskqacontract.Scope) ([]string, string) {
	// Disabling rename folding includes both deleted sources and added targets.
	diff, err := riskGit(root, "diff", "--no-ext-diff", "--no-textconv", "--no-renames", "--name-only", "-z", scope.BaseSHA, scope.HeadSHA, "--")
	if err != nil {
		return nil, "risk QA committed range unavailable: " + err.Error()
	}
	return strings.Split(string(diff), "\x00"), ""
}

// riskStatusPaths lists the working-tree paths from NUL-separated git status,
// including both sides of renames and copies.
func riskStatusPaths(root string) ([]string, string) {
	status, err := riskGit(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, "git status unavailable: " + err.Error()
	}
	var paths []string
	entries := strings.Split(string(status), "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if entry == "" {
			continue
		}
		if len(entry) < 4 || entry[2] != ' ' {
			return nil, "invalid NUL git status record"
		}
		paths = append(paths, entry[3:])
		if strings.ContainsAny(entry[:2], "RC") {
			i++
			if i >= len(entries) || entries[i] == "" {
				return nil, "missing NUL git rename source"
			}
			paths = append(paths, entries[i])
		}
	}
	return paths, ""
}

func riskGit(root string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gitbin.Path(), append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("git observation timed out: %w", ctx.Err())
	}
	return out, err
}
