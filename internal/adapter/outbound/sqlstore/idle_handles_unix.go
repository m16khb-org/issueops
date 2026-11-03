//go:build unix

package sqlstore

import "database/sql"

// releaseIdleFileHandles keeps the pool default on Unix, where an open SQLite
// file does not stop another process from removing its state root.
func releaseIdleFileHandles(*sql.DB) {}
