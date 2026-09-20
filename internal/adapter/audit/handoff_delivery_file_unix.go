//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package audit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type handoffDeliveryAuditMode int

const (
	handoffDeliveryAuditRead handoffDeliveryAuditMode = iota
	handoffDeliveryAuditAppend
)

// openHandoffDeliveryAudit walks from the filesystem root with openat and
// O_NOFOLLOW, then retains every directory handle from the trusted filesystem
// root through the state root, audit directory, and leaf. The caller reads back
// an append through the returned leaf descriptor; VerifyPath rejects any
// namespace-link replacement before success is reported.
func openHandoffDeliveryAudit(stateRoot string, mode handoffDeliveryAuditMode) (*handoffDeliveryAuditHandle, error) {
	stateRoot, err := handoffDeliveryStateRootPath(stateRoot)
	if err != nil {
		return nil, err
	}
	statePathFDs, statePathNames, err := openHandoffDeliveryStateRootUnix(stateRoot)
	if err != nil {
		return nil, err
	}
	stateFD := statePathFDs[len(statePathFDs)-1]
	closeState := true
	defer func() {
		if closeState {
			_ = closeHandoffDeliveryUnixFDs(statePathFDs)
		}
	}()

	if mode == handoffDeliveryAuditAppend {
		if err := unix.Mkdirat(stateFD, "audit", 0o700); err != nil && !errors.Is(err, unix.EEXIST) {
			return nil, fmt.Errorf("create handoff delivery audit directory: %w", err)
		}
	}
	auditFD, err := openPinnedHandoffDeliveryUnixDirectory(stateFD, "audit", nil)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit directory: %w", err)
	}
	closeAudit := true
	defer func() {
		if closeAudit {
			_ = unix.Close(auditFD)
		}
	}()
	if err := validateHandoffDeliveryUnixFD(auditFD, true, 0o700); err != nil {
		return nil, fmt.Errorf("handoff delivery audit directory: %w", err)
	}
	var beforeFile unix.Stat_t
	beforeFileErr := unix.Fstatat(auditFD, "handoff-delivery.jsonl", &beforeFile, unix.AT_SYMLINK_NOFOLLOW)
	if beforeFileErr != nil && !(mode == handoffDeliveryAuditAppend && errors.Is(beforeFileErr, unix.ENOENT)) {
		return nil, fmt.Errorf("inspect handoff delivery audit log: %w", beforeFileErr)
	}
	if beforeFileErr == nil && (beforeFile.Mode&unix.S_IFMT != unix.S_IFREG || beforeFile.Mode&0o777 != 0o600) {
		return nil, errors.New("handoff delivery audit log has unsafe type or permissions")
	}
	handoffDeliveryAuditBeforeLeafOpen()

	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	if mode == handoffDeliveryAuditAppend {
		flags = unix.O_RDWR | unix.O_APPEND | unix.O_CREAT | unix.O_CLOEXEC | unix.O_NOFOLLOW
		if errors.Is(beforeFileErr, unix.ENOENT) {
			flags |= unix.O_EXCL
		}
	}
	fileFD, err := unix.Openat(auditFD, "handoff-delivery.jsonl", flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit log: %w", err)
	}
	file := os.NewFile(uintptr(fileFD), filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl"))
	if file == nil {
		_ = unix.Close(fileFD)
		return nil, errors.New("open handoff delivery audit log: invalid file descriptor")
	}
	if err := validateHandoffDeliveryUnixFile(file, false, 0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("handoff delivery audit log: %w", err)
	}
	if beforeFileErr == nil {
		var openedFile unix.Stat_t
		if err := unix.Fstat(fileFD, &openedFile); err != nil || !sameHandoffDeliveryUnixObject(beforeFile, openedFile) {
			_ = file.Close()
			return nil, errors.New("handoff delivery audit log changed while opening")
		}
	}
	handoffDeliveryAuditAfterLeafOpen()

	handle := &handoffDeliveryAuditHandle{file: file}
	handle.verifyPath = func() error {
		for index, name := range statePathNames {
			if err := verifyHandoffDeliveryUnixEntry(statePathFDs[index], name, statePathFDs[index+1], true); err != nil {
				return fmt.Errorf("handoff delivery state path component %q changed: %w", name, err)
			}
		}
		if err := verifyHandoffDeliveryUnixEntry(stateFD, "audit", auditFD, true); err != nil {
			return fmt.Errorf("handoff delivery audit directory changed: %w", err)
		}
		if err := verifyHandoffDeliveryUnixEntry(auditFD, "handoff-delivery.jsonl", int(file.Fd()), false); err != nil {
			return fmt.Errorf("handoff delivery audit log changed: %w", err)
		}
		return nil
	}
	handle.closePath = func() error {
		return errors.Join(unix.Close(auditFD), closeHandoffDeliveryUnixFDs(statePathFDs))
	}
	closeAudit = false
	closeState = false
	return handle, nil
}

func openHandoffDeliveryStateRootUnix(stateRoot string) ([]int, []string, error) {
	if !filepath.IsAbs(stateRoot) {
		return nil, nil, errors.New("handoff delivery state root must be absolute")
	}
	parts := strings.FieldsFunc(filepath.Clean(stateRoot), func(r rune) bool { return r == os.PathSeparator })
	if len(parts) == 0 {
		return nil, nil, errors.New("handoff delivery state root cannot be the filesystem root")
	}
	currentFD, err := unix.Open(string(os.PathSeparator), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open handoff delivery trusted root: %w", err)
	}
	pathFDs := []int{currentFD}
	for index, part := range parts {
		var beforeOpen func()
		if index == len(parts)-1 {
			beforeOpen = handoffDeliveryAuditBeforeStateRootOpen
		}
		nextFD, openErr := openPinnedHandoffDeliveryUnixDirectory(currentFD, part, beforeOpen)
		if openErr != nil {
			_ = closeHandoffDeliveryUnixFDs(pathFDs)
			return nil, nil, fmt.Errorf("open handoff delivery state component %q: %w", part, openErr)
		}
		pathFDs = append(pathFDs, nextFD)
		currentFD = nextFD
	}
	return pathFDs, parts, nil
}

func closeHandoffDeliveryUnixFDs(fds []int) error {
	errs := make([]error, 0, len(fds))
	for index := len(fds) - 1; index >= 0; index-- {
		if err := unix.Close(fds[index]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func openPinnedHandoffDeliveryUnixDirectory(parentFD int, name string, beforeOpen func()) (int, error) {
	var before unix.Stat_t
	if err := unix.Fstatat(parentFD, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return -1, err
	}
	if before.Mode&unix.S_IFMT != unix.S_IFDIR {
		return -1, errors.New("not a real directory")
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	openedFD, err := unix.Openat(parentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return -1, err
	}
	var opened unix.Stat_t
	if err := unix.Fstat(openedFD, &opened); err != nil || !sameHandoffDeliveryUnixObject(before, opened) {
		_ = unix.Close(openedFD)
		return -1, errors.New("directory changed while opening")
	}
	return openedFD, nil
}

func sameHandoffDeliveryUnixObject(left, right unix.Stat_t) bool {
	return left.Dev == right.Dev && left.Ino == right.Ino && left.Mode&unix.S_IFMT == right.Mode&unix.S_IFMT
}

func validateHandoffDeliveryUnixFD(fd int, directory bool, mode os.FileMode) error {
	duplicate, err := unix.Dup(fd)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(duplicate), "handoff-delivery-audit-component")
	if file == nil {
		_ = unix.Close(duplicate)
		return errors.New("invalid file descriptor")
	}
	defer file.Close()
	return validateHandoffDeliveryUnixFile(file, directory, mode)
}

func validateHandoffDeliveryUnixFile(file *os.File, directory bool, mode os.FileMode) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if directory != info.IsDir() || (!directory && !info.Mode().IsRegular()) || info.Mode().Perm() != mode {
		return errors.New("unsafe type or permissions")
	}
	return nil
}

func verifyHandoffDeliveryUnixEntry(parentFD int, name string, openedFD int, directory bool) error {
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	if directory {
		flags |= unix.O_DIRECTORY
	}
	currentFD, err := unix.Openat(parentFD, name, flags, 0)
	if err != nil {
		return err
	}
	current := os.NewFile(uintptr(currentFD), name)
	if current == nil {
		_ = unix.Close(currentFD)
		return errors.New("invalid namespace file descriptor")
	}
	defer current.Close()
	openedDuplicate, err := unix.Dup(openedFD)
	if err != nil {
		return err
	}
	opened := os.NewFile(uintptr(openedDuplicate), name)
	if opened == nil {
		_ = unix.Close(openedDuplicate)
		return errors.New("invalid pinned file descriptor")
	}
	defer opened.Close()
	currentInfo, err := current.Stat()
	if err != nil {
		return err
	}
	openedInfo, err := opened.Stat()
	if err != nil {
		return err
	}
	if directory != currentInfo.IsDir() || (!directory && !currentInfo.Mode().IsRegular()) || !os.SameFile(currentInfo, openedInfo) {
		return errors.New("namespace entry no longer names the pinned object")
	}
	return nil
}
