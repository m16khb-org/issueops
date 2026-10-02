package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"

	authorityport "issueops/internal/port/authority"
)

type recordGuardKey struct{}

type recordGuardFunc = func(context.Context, authorityport.RecordReader) (context.Context, error)

type recordGuard struct {
	root string
	bind recordGuardFunc
}

var _ authorityport.RecordReader = (*DB)(nil)

var errRecordGuardContext = errors.New("sqlstore record guard must return a context derived from the span context")

// WithRecordGuard는 root가 보호하는 state root(grant database)에만 binder를 건다.
// 그 root의 span은 lock을 잡은 뒤 같은 root의 DB를 reader로 binder를 호출하고
// 반환된 context로 callback을 실행한다. 실패하면 callback과 data write 없이 lock을
// 풀고 그 오류를 반환한다. 그 root의 data write(Apply, CompareAndApply,
// CompareAndApplyFunc)는 mutation을 적용한 뒤 commit 직전에 같은 data transaction을
// reader로 binder를 다시 호출해, grant 회전·삭제와 write가 SQLite write lock으로
// 직렬화되게 한다. 다른 root의 span과 write는 guard를 그대로 통과시킨다.
func WithRecordGuard(ctx context.Context, root string, bind func(context.Context, authorityport.RecordReader) (context.Context, error)) context.Context {
	if ctx == nil || bind == nil {
		return ctx
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return context.WithValue(ctx, recordGuardKey{}, recordGuard{root: filepath.Clean(root), bind: bind})
}

func (d *DB) recordGuard(ctx context.Context) recordGuardFunc {
	guard, _ := ctx.Value(recordGuardKey{}).(recordGuard)
	if guard.root != d.dir {
		return nil
	}
	return guard.bind
}

func (d *DB) bindRecordGuard(spanCtx context.Context, run *spanRun) (context.Context, error) {
	bind := d.recordGuard(spanCtx)
	if bind == nil {
		return spanCtx, nil
	}
	bound, err := bind(spanCtx, d)
	if err != nil {
		return nil, err
	}
	if bound == nil || activeSpanRun(bound, d) != run {
		return nil, errRecordGuardContext
	}
	return bound, nil
}

// recheckRecordGuard는 이미 mutation을 적용한 data transaction을 reader로 이 root의
// guard를 다시 실행한다. 실패하면 호출자가 commit하지 않고 rollback한다.
func (d *DB) recheckRecordGuard(ctx context.Context, tx *sql.Tx) error {
	bind := d.recordGuard(ctx)
	if bind == nil {
		return nil
	}
	_, err := bind(ctx, txRecordReader{ctx: ctx, tx: tx})
	return err
}

type txRecordReader struct {
	ctx context.Context
	tx  *sql.Tx
}

func (r txRecordReader) Get(bucket, id string) ([]byte, bool, error) {
	var data []byte
	err := r.tx.QueryRowContext(r.ctx, `SELECT data FROM records WHERE bucket = ? AND id = ?`, bucket, id).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}
