//go:build windows

package audit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type handoffDeliveryAuditMode int

const (
	handoffDeliveryAuditRead handoffDeliveryAuditMode = iota
	handoffDeliveryAuditAppend
)

// Windows os.Root operations use handle-relative NtCreateFile opens. We reject
// every observed reparse component before and after each nested OpenRoot/OpenFile
// and bind the opened handle with os.SameFile, so a race is never accepted as a
// different state, audit, or leaf object.
func openHandoffDeliveryAudit(stateRoot string, mode handoffDeliveryAuditMode) (*handoffDeliveryAuditHandle, error) {
	stateRoot, err := handoffDeliveryStateRootPath(stateRoot)
	if err != nil {
		return nil, err
	}
	stateParent, stateName, state, err := openHandoffDeliveryStateRootWindows(stateRoot)
	if err != nil {
		return nil, err
	}
	closeState := true
	defer func() {
		if closeState {
			_ = state.Close()
			_ = stateParent.Close()
		}
	}()

	if mode == handoffDeliveryAuditAppend {
		if err := state.Mkdir("audit", 0o700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("create handoff delivery audit directory: %w", err)
		}
	}
	audit, err := openHandoffDeliveryWindowsDirectory(state, "audit", nil)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit directory: %w", err)
	}
	closeAudit := true
	defer func() {
		if closeAudit {
			_ = audit.Close()
		}
	}()
	flags := os.O_RDONLY
	if mode == handoffDeliveryAuditAppend {
		flags = os.O_CREATE | os.O_RDWR | os.O_APPEND
	}
	before, beforeErr := audit.Lstat("handoff-delivery.jsonl")
	if beforeErr != nil && !(mode == handoffDeliveryAuditAppend && errors.Is(beforeErr, os.ErrNotExist)) {
		return nil, beforeErr
	}
	if beforeErr == nil && !validHandoffDeliveryWindowsInfo(before, false) {
		return nil, errors.New("handoff delivery audit log has unsafe type")
	}
	handoffDeliveryAuditBeforeLeafOpen()
	if mode == handoffDeliveryAuditAppend && errors.Is(beforeErr, os.ErrNotExist) {
		flags |= os.O_EXCL
	}
	file, err := audit.OpenFile("handoff-delivery.jsonl", flags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open handoff delivery audit log: %w", err)
	}
	fileInfo, err := file.Stat()
	if err != nil || !validHandoffDeliveryWindowsInfo(fileInfo, false) {
		_ = file.Close()
		return nil, errors.New("handoff delivery audit log has unsafe type or permissions")
	}
	after, err := audit.Lstat("handoff-delivery.jsonl")
	if err != nil || !validHandoffDeliveryWindowsInfo(after, false) || !os.SameFile(after, fileInfo) || (beforeErr == nil && !os.SameFile(before, fileInfo)) {
		_ = file.Close()
		return nil, errors.New("handoff delivery audit log changed while opening")
	}
	handoffDeliveryAuditAfterLeafOpen()

	handle := &handoffDeliveryAuditHandle{file: file}
	handle.verifyPath = func() error {
		if err := verifyHandoffDeliveryWindowsRootEntry(stateParent, stateName, state); err != nil {
			return fmt.Errorf("handoff delivery state root changed: %w", err)
		}
		if err := verifyHandoffDeliveryWindowsRootEntry(state, "audit", audit); err != nil {
			return fmt.Errorf("handoff delivery audit directory changed: %w", err)
		}
		current, err := audit.Lstat("handoff-delivery.jsonl")
		if err != nil || !validHandoffDeliveryWindowsInfo(current, false) {
			return errors.New("handoff delivery audit log changed")
		}
		opened, err := file.Stat()
		if err != nil || !os.SameFile(current, opened) {
			return errors.New("handoff delivery audit log no longer names the pinned object")
		}
		return nil
	}
	handle.closePath = func() error { return errors.Join(audit.Close(), state.Close(), stateParent.Close()) }
	closeAudit = false
	closeState = false
	return handle, nil
}

func openHandoffDeliveryStateRootWindows(stateRoot string) (*os.Root, string, *os.Root, error) {
	volume := filepath.VolumeName(stateRoot)
	if volume == "" {
		return nil, "", nil, errors.New("handoff delivery state root has no volume")
	}
	rootPath := volume + string(os.PathSeparator)
	parts := strings.FieldsFunc(strings.TrimPrefix(stateRoot, rootPath), func(r rune) bool { return os.IsPathSeparator(uint8(r)) })
	if len(parts) == 0 {
		return nil, "", nil, errors.New("handoff delivery state root cannot be a volume root")
	}
	current, err := os.OpenRoot(rootPath)
	if err != nil {
		return nil, "", nil, err
	}
	for index, part := range parts {
		var beforeOpen func()
		if index == len(parts)-1 {
			beforeOpen = handoffDeliveryAuditBeforeStateRootOpen
		}
		next, err := openHandoffDeliveryWindowsDirectory(current, part, beforeOpen)
		if err != nil {
			_ = current.Close()
			return nil, "", nil, err
		}
		if index == len(parts)-1 {
			return current, part, next, nil
		}
		_ = current.Close()
		current = next
	}
	panic("unreachable")
}

func openHandoffDeliveryWindowsDirectory(parent *os.Root, name string, beforeOpen func()) (*os.Root, error) {
	before, err := parent.Lstat(name)
	if err != nil || !validHandoffDeliveryWindowsInfo(before, true) {
		return nil, errors.New("handoff delivery directory component is unsafe")
	}
	if beforeOpen != nil {
		beforeOpen()
	}
	next, err := parent.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, err := next.Stat(".")
	if err != nil {
		_ = next.Close()
		return nil, err
	}
	after, err := parent.Lstat(name)
	if err != nil || !validHandoffDeliveryWindowsInfo(after, true) || !os.SameFile(after, opened) || !os.SameFile(before, opened) {
		_ = next.Close()
		return nil, errors.New("handoff delivery directory changed while opening")
	}
	return next, nil
}

func verifyHandoffDeliveryWindowsRootEntry(parent *os.Root, name string, opened *os.Root) error {
	current, err := parent.Lstat(name)
	if err != nil || !validHandoffDeliveryWindowsInfo(current, true) {
		return errors.New("directory namespace entry is unsafe")
	}
	openedInfo, err := opened.Stat(".")
	if err != nil || !os.SameFile(current, openedInfo) {
		return errors.New("directory namespace entry no longer names the pinned object")
	}
	return nil
}

func validHandoffDeliveryWindowsInfo(info os.FileInfo, directory bool) bool {
	if info == nil {
		return false
	}
	// Windows reports ordinary directories and files as 0777 and 0666;
	// FileMode does not expose their ACL. The handle-relative no-follow opens,
	// reparse classification, and SameFile checks provide the path boundary.
	return validHandoffDeliveryWindowsMode(info.Mode(), directory)
}
