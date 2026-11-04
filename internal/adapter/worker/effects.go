// Package worker is the outbound adapter that persists worker jobs in the shared
// SQLite store and supplies the worker application service with its effects:
// directory resolution, locking, clock, process id, command execution, and
// liveness checks.
package worker

import (
	"context"
	"errors"
	policycontract "issueops/internal/contract/policy"
	"os"
	"path/filepath"
	"time"
)

// Store keeps public paths and filesystem paths fixed for one worker instance.
// Directory preserves the existing response path when the state override is relative.
type Store struct {
	Directory           string
	DirectoryError      error
	FilesystemDirectory string
	OpenDatabase        func(string) (StateDatabase, error)
	RunCommand          func(policycontract.CommandPolicyRequest) policycontract.CommandRunResult
}

// Dir returns the worker directory reported in responses, or the error captured
// when it was resolved.
func (store Store) Dir() (string, error) { return store.Directory, store.DirectoryError }

// EnsureDir creates the worker filesystem directory with owner-only permissions.
// A path error is reported relative to the public Directory.
func (store Store) EnsureDir(_ string) error {
	err := os.MkdirAll(store.FilesystemDirectory, 0o700)
	// Keep the response path relative while executing against the captured directory.
	if pathErr, ok := errors.AsType[*os.PathError](err); ok && !filepath.IsAbs(store.Directory) {
		if rel, relErr := filepath.Rel(store.FilesystemDirectory, pathErr.Path); relErr == nil {
			return &os.PathError{Op: pathErr.Op, Path: filepath.Join(store.Directory, rel), Err: pathErr.Err}
		}
	}
	return err
}

// WithLock runs fn inside the store's span lock. The directory and key arguments
// are ignored because the lock is scoped to the store itself.
func (store Store) WithLock(ctx context.Context, _, _ string, fn func(context.Context) error) error {
	db, err := store.open()
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}

// Now returns the current time.
func (Store) Now() time.Time { return time.Now() }

// PID returns the current process id.
func (Store) PID() int { return os.Getpid() }

// Run executes request through the injected command runner.
func (store Store) Run(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return store.RunCommand(request)
}

// ListIDs returns the ids of all stored worker jobs. The directory argument is
// ignored.
func (store Store) ListIDs(_ string) ([]string, error) {
	db, err := store.open()
	if err != nil {
		return nil, err
	}
	return db.List(workerBucket)
}

// PIDAlive reports whether a process with pid exists.
func (Store) PIDAlive(pid int) bool { return isPIDAlive(pid) }
