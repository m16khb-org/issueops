package authority

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	contract "issueops/internal/contract/authority"
	authorityport "issueops/internal/port/authority"
)

// ScopeResolver canonicalizes a request workspace and its repository identity.
// Git scopes are the common-dir shared by every worktree of one repository; a
// non-Git workspace is bounded by its own canonical root.
type ScopeResolver struct{ Git string }

var _ authorityport.ScopeResolver = ScopeResolver{}

func (r ScopeResolver) Resolve(ctx context.Context, workspaceRoot, cwd string) (contract.Scope, error) {
	if err := ctx.Err(); err != nil {
		return contract.Scope{}, err
	}
	root, err := canonicalDir(workspaceRoot, "workspace_root")
	if err != nil {
		return contract.Scope{}, err
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = root
	}
	resolvedCWD, err := canonicalDir(cwd, "cwd")
	if err != nil {
		return contract.Scope{}, err
	}
	if !inside(resolvedCWD, root) {
		return contract.Scope{}, fmt.Errorf("cwd is outside workspace_root")
	}
	scope := contract.Scope{WorkspaceRoot: root, CWD: resolvedCWD, SourceRoot: root}
	commonDir, topLevel, isGit, err := r.gitIdentity(ctx, root)
	if err != nil || !isGit {
		return scope, err
	}
	if scope.GitCommonDir, err = canonicalDir(commonDir, "git common-dir"); err != nil {
		return contract.Scope{}, err
	}
	if scope.SourceRoot, err = canonicalDir(topLevel, "git top-level"); err != nil {
		return contract.Scope{}, err
	}
	if !inside(root, scope.SourceRoot) {
		return contract.Scope{}, fmt.Errorf("workspace_root is outside its git work tree")
	}
	return scope, nil
}

func (r ScopeResolver) gitIdentity(ctx context.Context, root string) (string, string, bool, error) {
	git := r.Git
	if git == "" {
		git = "git"
	}
	command := exec.CommandContext(ctx, git, "-C", root, "rev-parse", "--path-format=absolute", "--git-common-dir", "--show-toplevel")
	command.Env = withoutGitEnvironment(os.Environ())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return "", "", false, ctxErr
		}
		if _, exited := errors.AsType[*exec.ExitError](err); exited && strings.Contains(stderr.String(), "not a git repository") {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("resolve git identity: %w", err)
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || lines[0] == "" || lines[1] == "" {
		return "", "", false, fmt.Errorf("resolve git identity: unexpected rev-parse output")
	}
	return lines[0], lines[1], true, nil
}

func withoutGitEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "GIT_") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func canonicalDir(path, label string) (string, error) {
	path = strings.TrimSpace(path)
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%s must be absolute", label)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s must be a directory", label)
	}
	return resolved, nil
}

func inside(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}
