//go:build windows

package sqlstore

import "database/sql"

// releaseIdleFileHandles closes each connection when it returns to the pool.
// Windows cannot delete or rename a file another handle holds open, so a
// cached idle connection would keep the state root, its WAL, and its parent
// directories from being removed by any process. Each operation reopens the
// files instead.
func releaseIdleFileHandles(db *sql.DB) { db.SetMaxIdleConns(0) }
