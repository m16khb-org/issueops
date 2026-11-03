//go:build windows

package audit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"issueops/internal/adapter/outbound/osfile"
)

type handoffDeliveryAuditMode int

const (
	handoffDeliveryAuditRead handoffDeliveryAuditMode = iota
	handoffDeliveryAuditAppend
)

// pinnedHandoffDeliveryEntry is an open handle and the path it was opened by.
// Go opens Windows files and directories without FILE_SHARE_DELETE, so while
// the handle is open the entry cannot be renamed, removed, or replaced.
type pinnedHandoffDeliveryEntry struct {
	path string
	file *os.File
}

// openHandoffDeliveryAudit pins every directory from the volume root through
// the state root and audit directory, then the leaf. The pins replace the Unix
// openat walk: no pinned component can be swapped for a link while the caller
// appends and reads back, and VerifyPath still rechecks each path's identity.
func openHandoffDeliveryAudit(stateRoot string, mode handoffDeliveryAuditMode, hooks handoffDeliveryOpenHooks) (*handoffDeliveryAuditHandle, error) {
	stateRoot, err := handoffDeliveryStateRootPath(stateRoot)
	if err != nil {
		return nil, err
	}
	pinned, err := pinHandoffDeliveryStateRoot(stateRoot, hooks.beforeStateRootOpen)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = closePinnedHandoffDelivery(pinned)
		}
	}()
	auditPath := filepath.Join(stateRoot, "audit")
	if mode == handoffDeliveryAuditAppend {
		if err := os.Mkdir(auditPath, 0o700); err == nil {
			if err := osfile.Chmod(auditPath, 0o700); err != nil {
				return nil, fmt.Errorf("create handoff delivery audit directory: %w", err)
			}
		} else if !errors.Is(err, fs.ErrExist) {
			return nil, fmt.Errorf("create handoff delivery audit directory: %w", err)
		}
	}
	audit, err := pinHandoffDeliveryEntry(auditPath, true, nil)
	if err != nil {
		return nil, err
	}
	pinned = append(pinned, audit)
	if err := validateHandoffDeliveryWindowsEntry(audit, true, 0o700); err != nil {
		return nil, fmt.Errorf("handoff delivery audit directory: %w", err)
	}
	file, err := openHandoffDeliveryAuditLeafWindows(filepath.Join(auditPath, "handoff-delivery.jsonl"), mode, hooks)
	if err != nil {
		return nil, err
	}
	leaf := pinnedHandoffDeliveryEntry{path: file.Name(), file: file}
	handle := &handoffDeliveryAuditHandle{file: file}
	handle.verifyPath = func() error {
		for _, entry := range append(slices.Clone(pinned), leaf) {
			if err := verifyPinnedHandoffDeliveryEntry(entry); err != nil {
				return fmt.Errorf("handoff delivery audit path component %q changed: %w", entry.path, err)
			}
		}
		return nil
	}
	handle.closePath = func() error { return closePinnedHandoffDelivery(pinned) }
	keep = true
	return handle, nil
}

// openHandoffDeliveryAuditLeafWindows checks the leaf before opening, opens
// it, and verifies that the opened file is the inspected owner-only file.
func openHandoffDeliveryAuditLeafWindows(path string, mode handoffDeliveryAuditMode, hooks handoffDeliveryOpenHooks) (*os.File, error) {
	before, beforeErr := os.Lstat(path)
	if beforeErr != nil && !(mode == handoffDeliveryAuditAppend && errors.Is(beforeErr, fs.ErrNotExist)) {
		return nil, fmt.Errorf("inspect handoff delivery audit log: %w", beforeErr)
	}
	if beforeErr == nil && (!before.Mode().IsRegular() || osfile.Perm(path, before) != 0o600) {
		return nil, errors.New("handoff delivery audit log has unsafe type or permissions")
	}
	// os.SameFile on a Lstat result reads its identity lazily from the path on
	// Windows, so a swap after inspection would go unnoticed; capture it now.
	var beforeIdentity osfile.Identity
	if beforeErr == nil {
		identity, ok := osfile.PathIdentity(path, before)
		if !ok {
			return nil, errors.New("handoff delivery audit log identity is unavailable")
		}
		beforeIdentity = identity
	}
	if hooks.beforeLeafOpen != nil {
		hooks.beforeLeafOpen()
	}
	flags := os.O_RDONLY
	if mode == handoffDeliveryAuditAppend {
		flags = os.O_RDWR | os.O_APPEND | os.O_CREATE
		if beforeErr != nil {
			flags |= os.O_EXCL
		}
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit log: %w", err)
	}
	if beforeErr != nil {
		if err := osfile.Chmod(path, 0o600); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("open handoff delivery audit log: %w", err)
		}
	}
	entry := pinnedHandoffDeliveryEntry{path: path, file: file}
	if err := validateHandoffDeliveryWindowsEntry(entry, false, 0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("handoff delivery audit log: %w", err)
	}
	if beforeErr == nil {
		opened, err := file.Stat()
		openedIdentity, openedOK := osfile.FileIdentity(file, opened)
		if err != nil || !openedOK || openedIdentity.Device != beforeIdentity.Device || openedIdentity.Inode != beforeIdentity.Inode {
			_ = file.Close()
			return nil, errors.New("handoff delivery audit log changed while opening")
		}
	}
	if hooks.afterLeafOpen != nil {
		hooks.afterLeafOpen()
	}
	return file, nil
}

// pinHandoffDeliveryStateRoot pins the volume root and each directory below
// it down to stateRoot, calling beforeStateRootOpen before the last one.
func pinHandoffDeliveryStateRoot(stateRoot string, beforeStateRootOpen func()) ([]pinnedHandoffDeliveryEntry, error) {
	volume := filepath.VolumeName(stateRoot)
	rest := strings.TrimPrefix(stateRoot, volume)
	parts := strings.FieldsFunc(rest, func(r rune) bool { return r == os.PathSeparator })
	if volume == "" || len(parts) == 0 {
		return nil, errors.New("handoff delivery state root cannot be the filesystem root")
	}
	current := volume + string(os.PathSeparator)
	root, err := os.Open(current)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery trusted root: %w", err)
	}
	pinned := []pinnedHandoffDeliveryEntry{{path: current, file: root}}
	for index, part := range parts {
		current = filepath.Join(current, part)
		var beforeOpen func()
		if index == len(parts)-1 {
			beforeOpen = beforeStateRootOpen
		}
		entry, err := pinHandoffDeliveryEntry(current, true, beforeOpen)
		if err != nil {
			_ = closePinnedHandoffDelivery(pinned)
			return nil, fmt.Errorf("open handoff delivery state component %q: %w", part, err)
		}
		pinned = append(pinned, entry)
	}
	return pinned, nil
}

// pinHandoffDeliveryEntry opens path after checking that it is a real
// directory, not a symlink or junction, and that the handle names it.
func pinHandoffDeliveryEntry(path string, directory bool, beforeOpen func()) (pinnedHandoffDeliveryEntry, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return pinnedHandoffDeliveryEntry{}, err
	}
	if !realHandoffDeliveryEntry(before, directory) {
		return pinnedHandoffDeliveryEntry{}, errors.New("not a real directory")
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	file, err := os.Open(path)
	if err != nil {
		return pinnedHandoffDeliveryEntry{}, err
	}
	entry := pinnedHandoffDeliveryEntry{path: path, file: file}
	if err := verifyPinnedHandoffDeliveryEntry(entry); err != nil {
		_ = file.Close()
		return pinnedHandoffDeliveryEntry{}, errors.New("directory changed while opening")
	}
	return entry, nil
}

func realHandoffDeliveryEntry(info fs.FileInfo, directory bool) bool {
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 || info.IsDir() != directory {
		return false
	}
	return directory || info.Mode().IsRegular()
}

// verifyPinnedHandoffDeliveryEntry checks that the path still names the
// pinned object and is not a link.
func verifyPinnedHandoffDeliveryEntry(entry pinnedHandoffDeliveryEntry) error {
	current, err := os.Lstat(entry.path)
	if err != nil {
		return err
	}
	opened, err := entry.file.Stat()
	if err != nil {
		return err
	}
	if !realHandoffDeliveryEntry(current, opened.IsDir()) || !os.SameFile(current, opened) {
		return errors.New("namespace entry no longer names the pinned object")
	}
	return nil
}

func validateHandoffDeliveryWindowsEntry(entry pinnedHandoffDeliveryEntry, directory bool, mode os.FileMode) error {
	info, err := entry.file.Stat()
	if err != nil {
		return err
	}
	if directory != info.IsDir() || (!directory && !info.Mode().IsRegular()) || osfile.Perm(entry.path, info) != mode {
		return errors.New("unsafe type or permissions")
	}
	return nil
}

func closePinnedHandoffDelivery(entries []pinnedHandoffDeliveryEntry) error {
	errs := make([]error, 0, len(entries))
	for _, entry := range slices.Backward(entries) {
		if err := entry.file.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
