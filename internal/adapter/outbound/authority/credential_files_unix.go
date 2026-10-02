//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package authority

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"

	domain "issueops/internal/domain/authority"
)

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
	fds, err := f.openGrantDirs(key, true)
	if err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	defer closeFDs(fds)
	keyFD := fds[len(fds)-1]
	fd, err := unix.Openat(keyFD, digest, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	file := os.NewFile(uintptr(fd), digest)
	written := false
	defer func() {
		if !written {
			_ = unix.Unlinkat(keyFD, digest, 0)
		}
	}()
	if err := writeCredential(file, token); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	if err := unix.Fsync(keyFD); err != nil {
		return "", fmt.Errorf("write authority credential: %w", err)
	}
	written = true
	return filepath.Join(f.grantsDir(), key, digest), nil
}

func writeCredential(file *os.File, token string) error {
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	if _, err := io.WriteString(file, token); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	return file.Close()
}

func (f CredentialFiles) Read(ctx context.Context, path string) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	key, digest, err := f.managedComponents(path)
	if err != nil {
		return "", "", err
	}
	fds, err := f.openGrantDirs(key, false)
	if err != nil {
		return "", "", errCredentialPath
	}
	defer closeFDs(fds)
	fd, err := unix.Openat(fds[len(fds)-1], digest, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return "", "", errCredentialPath
	}
	file := os.NewFile(uintptr(fd), digest)
	defer file.Close()
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil || !privateEntry(stat, unix.S_IFREG) {
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

// openGrantDirs pins state/mcp-http/grants/<key> with O_NOFOLLOW below the
// state directory and rejects any component that is not an owner-only directory.
func (f CredentialFiles) openGrantDirs(key string, create bool) ([]int, error) {
	stateFD, err := unix.Open(filepath.Clean(f.StateDir), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	fds := []int{stateFD}
	for _, name := range []string{"mcp-http", "grants", key} {
		if create {
			if err := unix.Mkdirat(fds[len(fds)-1], name, 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
				closeFDs(fds)
				return nil, err
			}
		}
		next, err := unix.Openat(fds[len(fds)-1], name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if err != nil {
			closeFDs(fds)
			return nil, err
		}
		fds = append(fds, next)
		var stat unix.Stat_t
		if err := unix.Fstat(next, &stat); err != nil || !privateEntry(stat, unix.S_IFDIR) {
			closeFDs(fds)
			return nil, fmt.Errorf("authority credential directory %q is not owner-only", name)
		}
	}
	return fds, nil
}

func privateEntry(stat unix.Stat_t, kind uint32) bool {
	return uint32(stat.Mode)&unix.S_IFMT == kind && uint32(stat.Mode)&0o077 == 0 && int(stat.Uid) == os.Geteuid()
}

func closeFDs(fds []int) {
	for index := len(fds) - 1; index >= 0; index-- {
		_ = unix.Close(fds[index])
	}
}
