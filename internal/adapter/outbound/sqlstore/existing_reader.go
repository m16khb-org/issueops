package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"issueops/internal/port"
)

// GetExisting은 state root, data 데이터베이스, span 데이터베이스, schema를
// 생성하지 않고 primary-key row 하나를 읽는다. read-only 연결은 SQLite
// 경합에서 최대 existingReadBusyTimeout만 대기하므로, lifecycle-hook 조회는
// 일시적 writer commit과 WAL checkpoint를 견디면서도 유한하게 끝난다.
func GetExisting(dir, bucket, id string) ([]byte, bool, error) {
	data, err := openExistingData(dir)
	if err != nil {
		return nil, false, err
	}
	defer data.Close()
	var raw []byte
	err = data.QueryRow(`SELECT data FROM records WHERE bucket = ? AND id = ?`, bucket, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return raw, true, nil
}

// ListExisting은 state 파일을 생성하거나 복구하지 않고 기존 data store에서
// bucket의 ID를 반환한다. store가 없으면 fs.ErrNotExist를 반환한다.
func ListExisting(dir, bucket string) ([]string, error) {
	data, err := openExistingData(dir)
	if err != nil {
		return nil, err
	}
	defer data.Close()
	rows, err := data.Query(`SELECT id FROM records WHERE bucket = ? ORDER BY id`, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GetAllExisting은 state 파일을 생성하거나 복구하지 않고 기존 data store에서
// bucket의 row를 반환한다. store가 없으면 fs.ErrNotExist를 반환한다.
func GetAllExisting(dir, bucket string) ([]port.RecordRow, error) {
	result := []port.RecordRow{}
	err := WalkExisting(context.Background(), dir, bucket, func(row port.RecordRow) error {
		result = append(result, row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// WalkExisting visits ordered rows without creating state or retaining the bucket.
// Each callback completes before the next row is read.
func WalkExisting(ctx context.Context, dir, bucket string, visit func(port.RecordRow) error) error {
	return WalkExistingAfter(ctx, dir, bucket, "", func(bool) string { return "" }, visit)
}

// WalkExistingAfter visits ordered rows whose id sorts after start(cursorExists)
// on one read-only connection. cursorExists reports whether bucket stores a row
// with id cursor; an empty cursor is not looked up. The caller owns the rule
// that maps the cursor to the range start.
func WalkExistingAfter(ctx context.Context, dir, bucket, cursor string, start func(cursorExists bool) string, visit func(port.RecordRow) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := openExistingData(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, data.Close()) }()
	cursorExists := false
	if cursor != "" {
		var present int
		switch err := data.QueryRowContext(ctx, `SELECT 1 FROM records WHERE bucket = ? AND id = ?`, bucket, cursor).Scan(&present); {
		case err == nil:
			cursorExists = true
		case !errors.Is(err, sql.ErrNoRows):
			return err
		}
	}
	rows, err := data.QueryContext(ctx, `SELECT id, data FROM records WHERE bucket = ? AND id > ? ORDER BY id`, bucket, start(cursorExists))
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var row port.RecordRow
		if err := rows.Scan(&row.ID, &row.Data); err != nil {
			return err
		}
		if err := visit(row); err != nil {
			return err
		}
	}
	return errors.Join(rows.Err(), ctx.Err())
}

func openExistingData(dir string) (*sql.DB, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("sqlstore existing open %q: %w", dir, err)
	}
	dataPath := filepath.Join(abs, dataDBFile)
	if _, err := os.Stat(dataPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("sqlstore existing data db %s: %w", abs, fs.ErrNotExist)
		}
		return nil, err
	}
	return openSQLite(dataPath, fmt.Sprintf("mode=ro&_pragma=busy_timeout(%d)&_pragma=query_only(1)", existingReadBusyTimeout/time.Millisecond))
}
