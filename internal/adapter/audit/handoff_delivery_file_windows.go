//go:build windows

package audit

import (
	"errors"
	"os"
	"path/filepath"
)

type handoffDeliveryAuditMode int

const (
	handoffDeliveryAuditRead handoffDeliveryAuditMode = iota
	handoffDeliveryAuditAppend
)

func openHandoffDeliveryAudit(stateRoot string, mode handoffDeliveryAuditMode) (*os.File, error) {
	stateRoot = filepath.Clean(stateRoot)
	for _, path := range []string{stateRoot, filepath.Join(stateRoot, "audit")} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) && mode == handoffDeliveryAuditAppend && path != stateRoot {
			if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
				return nil, err
			}
			info, err = os.Lstat(path)
		}
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || (path != stateRoot && info.Mode().Perm()&0o077 != 0) {
			return nil, errors.New("handoff delivery audit ancestor has unsafe type or permissions")
		}
	}
	path := filepath.Join(stateRoot, "audit", "handoff-delivery.jsonl")
	handoffDeliveryAuditBeforeLeafOpen()
	flags := os.O_RDONLY
	if mode == handoffDeliveryAuditAppend {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		file.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("handoff delivery audit log has unsafe type or permissions")
	}
	return file, nil
}
