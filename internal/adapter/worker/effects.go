package worker

import (
	"context"
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

func (store Store) Dir() (string, error) { return store.Directory, store.DirectoryError }
func (store Store) EnsureDir(_ string) error {
	err := os.MkdirAll(store.FilesystemDirectory, 0o700)
	// Keep the response path relative while executing against the captured directory.
	if pathErr, ok := err.(*os.PathError); ok && !filepath.IsAbs(store.Directory) {
		if rel, relErr := filepath.Rel(store.FilesystemDirectory, pathErr.Path); relErr == nil {
			return &os.PathError{Op: pathErr.Op, Path: filepath.Join(store.Directory, rel), Err: pathErr.Err}
		}
	}
	return err
}
func (store Store) WithLock(ctx context.Context, _, _ string, fn func(context.Context) error) error {
	db, err := store.open()
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}
func (Store) Now() time.Time { return time.Now() }
func (Store) PID() int       { return os.Getpid() }
func (store Store) Run(request policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
	return store.RunCommand(request)
}
func (store Store) ListIDs(_ string) ([]string, error) {
	db, err := store.open()
	if err != nil {
		return nil, err
	}
	return db.List(workerBucket)
}
func (Store) PIDAlive(pid int) bool { return isPIDAlive(pid) }
