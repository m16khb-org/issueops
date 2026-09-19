//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type handoffDeliveryAuditMode int

const (
	handoffDeliveryAuditRead handoffDeliveryAuditMode = iota
	handoffDeliveryAuditAppend
)

// openHandoffDeliveryAudit pins every component below stateRoot with openat and
// O_NOFOLLOW. The validation and the returned descriptor therefore refer to
// the same objects even if another process replaces path names concurrently.
func openHandoffDeliveryAudit(stateRoot string, mode handoffDeliveryAuditMode) (*os.File, error) {
	stateRoot = filepath.Clean(stateRoot)
	rootInfo, err := os.Lstat(stateRoot)
	if err != nil {
		return nil, err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, errors.New("handoff delivery state root must be a real directory")
	}
	rootFD, err := unix.Open(stateRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery state root: %w", err)
	}
	defer unix.Close(rootFD)

	if mode == handoffDeliveryAuditAppend {
		if err := unix.Mkdirat(rootFD, "audit", 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
			return nil, fmt.Errorf("create handoff delivery audit directory: %w", err)
		}
	}
	auditFD, err := unix.Openat(rootFD, "audit", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit directory: %w", err)
	}
	defer unix.Close(auditFD)
	var directoryStat unix.Stat_t
	if err := unix.Fstat(auditFD, &directoryStat); err != nil {
		return nil, err
	}
	if directoryStat.Mode&unix.S_IFMT != unix.S_IFDIR || directoryStat.Mode&0o077 != 0 {
		return nil, errors.New("handoff delivery audit directory has unsafe type or permissions")
	}
	handoffDeliveryAuditBeforeLeafOpen()

	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	if mode == handoffDeliveryAuditAppend {
		flags = unix.O_WRONLY | unix.O_APPEND | unix.O_CREAT | unix.O_CLOEXEC | unix.O_NOFOLLOW
	}
	fileFD, err := unix.Openat(auditFD, "handoff-delivery.jsonl", flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit log: %w", err)
	}
	var fileStat unix.Stat_t
	if err := unix.Fstat(fileFD, &fileStat); err != nil {
		unix.Close(fileFD)
		return nil, err
	}
	if fileStat.Mode&unix.S_IFMT != unix.S_IFREG || fileStat.Mode&0o777 != 0o600 {
		unix.Close(fileFD)
		return nil, errors.New("handoff delivery audit log has unsafe type or permissions")
	}
	return os.NewFile(uintptr(fileFD), filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")), nil
}
