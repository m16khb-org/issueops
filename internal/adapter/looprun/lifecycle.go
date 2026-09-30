package looprun

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func (store Store) WithLock(ctx context.Context, loopID string, fn func(context.Context) error) error {
	if _, err := normalizeLoopID(loopID); err != nil {
		return err
	}
	db, err := store.open()
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}

type Identity struct {
	BaseDir   string
	BaseError error
}

func (identity Identity) NormalizeRepo(repo string) (string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("repo is required")
	}
	if filepath.IsAbs(repo) {
		return filepath.Clean(repo), nil
	}
	if identity.BaseError != nil {
		return "", identity.BaseError
	}
	return filepath.Join(identity.BaseDir, repo), nil
}
func (Identity) NormalizeID(id string) (string, error) { return normalizeLoopID(id) }
func (Identity) NewID(repo, name string) string        { return newLoopID(repo, name) }

type Clock struct{ Time func() time.Time }

func (clock Clock) Now() string { return clock.Time().UTC().Format(time.RFC3339Nano) }
