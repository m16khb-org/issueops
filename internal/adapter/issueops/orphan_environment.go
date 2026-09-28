package issueops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/outbound/sqlstore"
	app "issueops/internal/application/issueopscleanup"
	health "issueops/internal/contract/operationalhealth"
)

// OrphanEnvironment performs filesystem, local Git and record-lock operations.
// The composition root supplies a local-only inventory collector.
type OrphanEnvironment struct {
	StateRoot      string
	LocalInventory func(context.Context, string) (health.Snapshot, error)
}

func (s OrphanEnvironment) CanonicalPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" || strings.ContainsRune(path, 0) {
		return "", fmt.Errorf("path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(abs)
		if err == nil {
			return filepath.Join(append([]string{resolved}, suffix...)...), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", err
		}
		suffix = append([]string{filepath.Base(abs)}, suffix...)
		abs = parent
	}
}

func (s OrphanEnvironment) StateOutsideTarget(target string) (bool, error) {
	target, err := s.CanonicalPath(target)
	if err != nil {
		return false, err
	}
	root, err := s.CanonicalPath(s.StateRoot)
	if err != nil {
		return false, err
	}
	relative, err := filepath.Rel(target, root)
	if err != nil {
		return false, err
	}
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)), nil
}

func (s OrphanEnvironment) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	code, out := defaultExecutionSyncBaseGit(ctx, dir, args...)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if code != 0 {
		detail := strings.TrimSpace(out)
		if detail == "" {
			detail = "git returned a non-zero exit"
		}
		if len(detail) > 512 {
			detail = detail[:512]
		}
		return nil, fmt.Errorf("%s", detail)
	}
	return []byte(out), nil
}

func (s OrphanEnvironment) Clean(ctx context.Context, path string) (bool, error) {
	output, err := s.Run(ctx, path, "status", "--porcelain=v1")
	return strings.TrimSpace(string(output)) == "", err
}
func (s OrphanEnvironment) CollectLocal(ctx context.Context, repo string) (health.Snapshot, error) {
	return s.LocalInventory(ctx, repo)
}
func (s OrphanEnvironment) ExcludeWrites(ctx context.Context) (app.CleanupLifetime, error) {
	db, err := sqlstore.Open(s.StateRoot)
	if err != nil {
		return nil, err
	}
	lease, err := db.ExcludeWrites(ctx)
	if err != nil {
		return nil, err
	}
	return cleanupLifetime{lease}, nil
}
func (s OrphanEnvironment) RemoveWorktree(ctx context.Context, repo, path string) error {
	_, err := s.Run(ctx, repo, "worktree", "remove", path)
	if err != nil {
		return fmt.Errorf("remove confirmed local worktree: %w", err)
	}
	return nil
}
func (s OrphanEnvironment) DeleteBranch(ctx context.Context, repo, branch, head string) error {
	_, err := s.Run(ctx, repo, "update-ref", "-d", "refs/heads/"+branch, head)
	if err != nil {
		return fmt.Errorf("remove confirmed local branch with preview HEAD CAS: %w", err)
	}
	return nil
}
