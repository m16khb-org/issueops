//go:build windows

package authority

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"issueops/internal/adapter/outbound/osfile"
	domain "issueops/internal/domain/authority"
)

// Windows has no openat, so each component is checked by path: it must not be
// a reparse point and must be owner-only. The credential leaf is created with
// CREATE_NEW, which never follows or replaces an existing entry.

func (f CredentialFiles) Write(ctx context.Context, key, token string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	digest := domain.TokenDigest(token)
	if err := validateCredential(key, digest, token); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	if !filepath.IsAbs(f.StateDir) {
		return "", fmt.Errorf("write authority credential: state directory must be absolute")
	}
	keyDir, err := f.grantDir(key, true)
	if err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	path := filepath.Join(keyDir, digest)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	written := false
	defer func() {
		if !written {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	if err := osfile.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	if _, err := io.WriteString(file, token); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	written = true
	return path, nil
}

// Prune removes the key's credential files other than keepDigest. The caller
// runs it under the grant lock with the current record's digest.
func (f CredentialFiles) Prune(ctx context.Context, key, keepDigest string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !domain.ValidKey(key) {
		return errCredentialPath
	}
	keyDir, err := f.grantDir(key, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errCredentialPath
	}
	entries, err := os.ReadDir(keyDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() != keepDigest {
			if err := os.Remove(filepath.Join(keyDir, entry.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}
	return nil
}

func (f CredentialFiles) Read(ctx context.Context, path string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	key, digest, err := f.managedComponents(path)
	if err != nil {
		return "", "", err
	}
	keyDir, err := f.grantDir(key, false)
	if err != nil {
		return "", "", errCredentialPath
	}
	path = filepath.Join(keyDir, digest)
	before, err := os.Lstat(path)
	if err != nil || !privateEntry(path, before, false) {
		return "", "", errCredentialPath
	}
	file, err := os.Open(path)
	if err != nil {
		return "", "", errCredentialPath
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return "", "", errCredentialPath
	}
	data, err := io.ReadAll(io.LimitReader(file, maxCredentialBytes+1))
	if err != nil || len(data) > maxCredentialBytes {
		return "", "", errCredentialPath
	}
	token := string(data)
	if err := validateCredential(key, digest, token); err != nil {
		return "", "", err
	}
	return key, token, nil
}

// grantDir checks state/mcp-http/grants/<key> below the state directory,
// creating missing components when create is set, and rejects any component
// that is not an owner-only directory.
func (f CredentialFiles) grantDir(key string, create bool) (string, error) {
	dir := filepath.Clean(f.StateDir)
	if info, err := os.Stat(dir); err != nil {
		return "", err
	} else if !info.IsDir() {
		return "", fmt.Errorf("authority credential state directory is not a directory")
	}
	for _, name := range []string{"mcp-http", "grants", key} {
		dir = filepath.Join(dir, name)
		if create {
			if err := os.Mkdir(dir, 0o700); err == nil {
				if err := osfile.Chmod(dir, 0o700); err != nil {
					return "", err
				}
			} else if !errors.Is(err, fs.ErrExist) {
				return "", err
			}
		}
		info, err := os.Lstat(dir)
		if err != nil {
			return "", err
		}
		if !privateEntry(dir, info, true) {
			return "", fmt.Errorf("authority credential directory %q is not owner-only", name)
		}
	}
	return dir, nil
}

func privateEntry(path string, info fs.FileInfo, directory bool) bool {
	if info.Mode()&os.ModeSymlink != 0 || info.IsDir() != directory || (!directory && !info.Mode().IsRegular()) {
		return false
	}
	return osfile.Perm(path, info)&0o077 == 0
}
