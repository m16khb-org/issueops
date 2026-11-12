//go:build windows

package sqlstore

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isTransientSidecarError reports whether a sidecar repair failed with
// ERROR_ACCESS_DENIED. SQLite deletes -wal and -shm when its last connection
// closes; while another process still holds the file, the name is
// delete-pending (STATUS_DELETE_PENDING) and every open or attribute query on
// it answers ERROR_ACCESS_DENIED until that handle closes. A real DACL denial
// returns the same error, so the caller retries only within a short bound and
// then fails closed.
func isTransientSidecarError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
