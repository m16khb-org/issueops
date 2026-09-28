// Package processlease holds an execution lifetime across inherited subprocesses.
// The stable lock file contains no task state or recovery authority.
package processlease

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

var ErrBusy = errors.New("execution lifetime is still active")

type Lease struct {
	file      *os.File
	directory string
	key       string
}

type contextKey struct{}

func (l *Lease) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, contextKey{}, l.file)
}

// Attach preserves existing extra descriptors. Children need only keep this
// descriptor open; they do not read or write it.
func Attach(ctx context.Context, cmd *exec.Cmd) {
	if file, ok := ctx.Value(contextKey{}).(*os.File); ok {
		cmd.ExtraFiles = append(cmd.ExtraFiles, file)
	}
}

// Close must not explicitly unlock: an inherited descriptor may still be live.
func (l *Lease) Close() error { return l.file.Close() }

// Drain closes our descriptor then tries a fresh acquisition. Success proves
// no inheriting command still owns the lock. No external calls may follow it.
// Another executor can win this gap; record finalization also requires its own
// attempt token and revision CAS, which this technical lock does not provide.
func (l *Lease) Drain(ctx context.Context) (*Lease, error) {
	before, err := l.file.Stat()
	if err != nil {
		return nil, err
	}
	if err := l.Close(); err != nil {
		return nil, err
	}
	next, err := Acquire(ctx, l.directory, l.key)
	if err != nil {
		return nil, err
	}
	after, err := next.file.Stat()
	if err != nil || !os.SameFile(before, after) {
		_ = next.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("execution lifetime lock identity changed")
	}
	return next, nil
}
