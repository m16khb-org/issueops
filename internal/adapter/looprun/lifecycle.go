package looprun

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// WithLock runs fn inside one store transaction span for loopID, so concurrent
// loop mutations are serialized. It fails before opening the store when loopID
// is invalid.
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

// Identity resolves repository paths and generates loop IDs. Relative repo
// names resolve against BaseDir; BaseError, when set, is returned instead for
// relative names because the base directory could not be determined.
type Identity struct {
	BaseDir   string
	BaseError error
}

// NormalizeRepo trims repo and returns it cleaned when absolute, or joined to
// BaseDir when relative. An empty repo, or a relative one with BaseError set,
// is an error.
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

// NormalizeID validates and normalizes a loop ID.
func (Identity) NormalizeID(id string) (string, error) { return normalizeLoopID(id) }

// NewID returns a new loop ID derived from repo and name.
func (Identity) NewID(repo, name string) string { return newLoopID(repo, name) }

// Clock formats the current time from its Time function.
type Clock struct{ Time func() time.Time }

// Now returns the current time in UTC as an RFC 3339 string with nanoseconds.
func (clock Clock) Now() string { return clock.Time().UTC().Format(time.RFC3339Nano) }
