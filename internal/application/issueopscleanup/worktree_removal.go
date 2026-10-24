package issueopscleanup

import (
	"context"
	"errors"
	"fmt"
)

// worktreeRemoval is the git-side removal of a cycle's linked worktree shared
// by cleanup finish and cleanup abandon.
type worktreeRemoval struct {
	Git         func(context.Context, string, ...string) (int, string)
	Directory   func(string) (bool, error)
	PruneParent func(string)
	// HonorCancel makes a failed removal under a canceled context return the
	// cancellation instead of observing the directory (abandon semantics).
	HonorCancel bool
}

// remove runs `git worktree remove`. A failed command still counts as removed
// when the directory is observably gone; then the empty `<repo>.worktrees`
// parent is pruned so finished cycles leave nothing behind.
func (w worktreeRemoval) remove(ctx context.Context, repo, root string) error {
	if code, out := w.Git(ctx, repo, "worktree", "remove", root); code != 0 {
		if err := ctx.Err(); w.HonorCancel && err != nil {
			return err
		}
		present, observeErr := w.Directory(root)
		if observeErr != nil || present {
			return errors.Join(fmt.Errorf("git worktree remove: %s", out), observeErr)
		}
	}
	if w.PruneParent != nil {
		w.PruneParent(root)
	}
	return nil
}

// unregisterMissing drops git's registration of a worktree whose directory is
// already gone. Probe: after a deleted worktree, finish and abandon removed
// the record and branch but git kept listing the path as prunable, so the
// same branch or path could not be added again. `git worktree remove` on a
// missing path only drops that entry; it runs only while the directory is
// still observably absent, so a directory recreated since the preview is
// never removed here.
func (w worktreeRemoval) unregisterMissing(ctx context.Context, repo, root string) {
	if root == "" {
		return
	}
	if present, err := w.Directory(root); err != nil || present {
		return
	}
	_, _ = w.Git(ctx, repo, "worktree", "remove", root)
	if w.PruneParent != nil {
		w.PruneParent(root)
	}
}
