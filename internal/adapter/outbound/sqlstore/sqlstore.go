// Package sqlstore는 issueops state root를 위한 공용 SQLite 기반 record store다.
// state root 디렉터리 하나는 SQLite 파일 두 개를 소유한다. issueops.db는 모든
// record를 (bucket, id, data-JSON) row로 보관하고, issueops.lock.db는 프로세스
// 간 span lock을 실어 나르기 위해서만 존재한다. read-modify-write span은
// 프로세스 안에서는 디렉터리별 token gate로, 프로세스 간에는 span이 지속되는
// 동안 lock 데이터베이스에 BEGIN IMMEDIATE 트랜잭션을 유지하는 방식으로
// 직렬화한다 — write lock은 프로세스와 함께 죽으므로, holder가 crash해도 이후
// 경쟁자가 deadlock에 빠질 수 없다. 데이터 write는 issueops.db에서 autocommit
// 되므로 span 자신의 write가 그 span과 동시 reader에게 계속 보이며, 이는 이전
// flock 기반 파일 레이아웃이 가졌던 가시성과 동일하다. Apply는 여러 data row를
// 한 트랜잭션으로 commit해야 하는 호출자를 위한 좁은 예외다.
package sqlstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"issueops/internal/port"
	stateport "issueops/internal/port/state"

	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	dataDBFile      = "issueops.db"
	spanDBFile      = "issueops.lock.db"
	spanLockMaxWait = 60 * time.Second
	// 짧은 cross-process span이 풀린 직후 10ms 고정 tick까지 기다리지 않도록
	// 1ms에서 시작하되, 장기 경합의 SQLite syscall 밀도는 기존 10ms로 제한한다.
	spanLockInitialRetryGap = time.Millisecond
	spanLockMaxRetryGap     = 10 * time.Millisecond
	// existingReadBusyTimeout은 read-only existing-store 조회가 일시적 SQLite
	// 경합(writer commit, daemon WAL checkpoint)에서 대기하는 시간의 상한이다.
	// 값이 0이면 밀리초 단위 checkpoint 구간 동안 lifecycle-hook 조회가 즉시
	// 실패해, 건강한 state에서도 mutation guard가 fail-closed됐다. 짧은 유한
	// 대기는 그런 허위 실패 없이 hook 응답성을 유지한다.
	existingReadBusyTimeout = 2 * time.Second
	openLockMaxWait         = 10 * time.Second
)

var sqliteFileSuffixes = [...]string{"", "-wal", "-shm", "-journal"}

// DB는 state root 디렉터리 하나에 대한 핸들이다.
type DB struct {
	dir      string
	data     *sql.DB
	span     *sql.DB
	spanGate chan struct{}
	// unattributedEpoch는 활성 span context에 귀속할 수 없는 write의 시작과 끝마다
	// 증가하고, unattributedInFlight는 그런 write가 진행 중인 동안 0이 아니다.
	// span은 lock 획득 전에 epoch→in-flight 순서로 표본을 떠고 lock 해제 뒤 epoch를
	// 다시 비교한다. baseline 전에 시작해 hold 중에 commit하는 write는 in-flight로,
	// 그 뒤에 시작한 write는 epoch 변화로 잡혀 coverage가 unknown이 된다.
	unattributedEpoch    atomic.Uint64
	unattributedInFlight atomic.Int64
	// hooks는 sleep 없이 경계 사이에 사건을 끼워 넣는 테스트 전용 seam이다.
	// production 핸들에서는 모두 nil이다.
	hooks dbTestHooks
}

type dbTestHooks struct {
	// unattributedWriteStarted는 귀속 불가 write가 시작을 기록한 뒤, 실행 전에 불린다.
	unattributedWriteStarted func()
	// unattributedWriteFinished는 귀속 불가 write가 끝난 뒤, 끝 기록 전에 불린다.
	unattributedWriteFinished func()
	// beforeDataCommit은 data commit의 취소 확인 직전에 불린다.
	beforeDataCommit func()
}

type spanChainKey struct{}

// NestedSpanError는 전파된 span chain에서 이미 활성인 root로 다시 진입하려는
// 시도를 보고한다.
type NestedSpanError struct {
	ActiveDirs   []string
	RequestedDir string
}

func (e *NestedSpanError) Error() string {
	return fmt.Sprintf("sqlstore nested span: root %q is already active in %v", e.RequestedDir, e.ActiveDirs)
}

type RawCASError struct {
	Bucket string
	ID     string
}

func (e *RawCASError) Error() string {
	return fmt.Sprintf("sqlstore raw CAS failed for row %s/%s", e.Bucket, e.ID)
}

func (e *RawCASError) FailedBucket() string { return e.Bucket }

var _ port.TransactionalRecordStore = (*DB)(nil)
var _ stateport.TransactionalStore = (*DB)(nil)
var _ stateport.Store = (*DB)(nil)

var (
	handles   = map[string]*DB{}
	handlesMu sync.Mutex
)

// Open은 dir에 대한 캐시된 핸들을 반환하며, 디렉터리와 두 SQLite 파일이 없으면
// 생성한다. 핸들은 절대 경로 디렉터리별로 캐시되므로 한 프로세스의 모든
// 호출자가 같은 in-process span mutex를 공유한다. 이미 제거된 root의 핸들은
// 다음 Open에서 닫고 축출해 임시 state root가 연결과 goroutine을 누적하지
// 않게 한다.
func Open(dir string) (*DB, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("sqlstore open %q: %w", dir, err)
	}
	handlesMu.Lock()
	defer handlesMu.Unlock()
	pruneRemovedHandlesLocked()
	if err := ensurePrivateRoot(abs); err != nil {
		return nil, fmt.Errorf("sqlstore secure root %s: %w", abs, err)
	}
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
		return nil, err
	}
	if d, ok := handles[abs]; ok {
		return d, nil
	}
	d, err := newDBWithRetry(abs)
	if err != nil {
		return nil, err
	}
	handles[abs] = d
	return d, nil
}

func pruneRemovedHandlesLocked() {
	for root, db := range handles {
		if _, err := os.Stat(root); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		delete(handles, root)
		_ = db.data.Close()
		_ = db.span.Close()
	}
}

func newDBWithRetry(abs string) (*DB, error) {
	deadline := time.NewTimer(openLockMaxWait)
	defer deadline.Stop()
	retry := time.NewTicker(spanLockMaxRetryGap)
	defer retry.Stop()
	for {
		db, err := newDB(abs)
		if err == nil || !isSQLiteLockContention(err) {
			return db, err
		}
		select {
		case <-deadline.C:
			return nil, err
		case <-retry.C:
		}
	}
}

// newDB는 캐시되지 않은 핸들을 연다. 테스트는 두 번째 uncached 핸들로 프로세스
// 간 직렬화가 공유 mutex가 아니라 SQLite에서 온다는 것을 증명한다.
func newDB(abs string) (*DB, error) {
	if err := ensurePrivateRoot(abs); err != nil {
		return nil, fmt.Errorf("sqlstore secure root %s: %w", abs, err)
	}
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
		return nil, err
	}
	dataPath := filepath.Join(abs, dataDBFile)
	if err := touchPrivate(dataPath); err != nil {
		return nil, fmt.Errorf("sqlstore create data db: %w", err)
	}
	data, err := openSQLite(dataPath, "_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("sqlstore open data db: %w", err)
	}
	if _, err := data.Exec(`CREATE TABLE IF NOT EXISTS records (
		bucket TEXT NOT NULL,
		id     TEXT NOT NULL,
		data   BLOB NOT NULL,
		PRIMARY KEY (bucket, id)
	) WITHOUT ROWID`); err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore init schema: %w", err)
	}
	spanPath := filepath.Join(abs, spanDBFile)
	if err := touchPrivate(spanPath); err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore create span db: %w", err)
	}
	// SQLite busy handler는 driver가 blocked BEGIN을 interrupt할 때 즉시
	// 반환하지 않는다. 여기서는 비활성화하고, beginSpanTx가 60초 상한을 유지한
	// 채 호출자 context에 select할 수 있는 typed retry를 수행하게 한다.
	span, err := openSQLite(spanPath, "_pragma=busy_timeout(0)&_txlock=immediate")
	if err != nil {
		data.Close()
		return nil, fmt.Errorf("sqlstore open span db: %w", err)
	}
	// span 데이터베이스에는 schema write가 한 번은 필요하다. 그래야 파일이
	// 실제로 존재하고 BEGIN IMMEDIATE가 lock할 진짜 데이터베이스가 생긴다.
	if _, err := span.Exec(`CREATE TABLE IF NOT EXISTS span (id INTEGER PRIMARY KEY CHECK (id = 1))`); err != nil {
		data.Close()
		span.Close()
		return nil, fmt.Errorf("sqlstore init span db: %w", err)
	}
	if _, err := repairPrivateSQLiteFiles(abs); err != nil {
		data.Close()
		span.Close()
		return nil, err
	}
	return &DB{dir: abs, data: data, span: span, spanGate: newSpanGate()}, nil
}

func newSpanGate() chan struct{} {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return gate
}

// touchPrivate는 path를 0600으로 미리 생성해, SQLite(와 데이터베이스 파일의
// mode를 물려받는 -wal/-shm sidecar)가 state를 더 넓은 권한으로 노출하지
// 못하게 한다.
func touchPrivate(path string) error {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular SQLite file %s", path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func ensurePrivateRoot(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("refusing non-directory state root %s", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func repairPrivateSQLiteFiles(dir string) ([]string, error) {
	var repaired []string
	for _, base := range [...]string{dataDBFile, spanDBFile} {
		for _, suffix := range sqliteFileSuffixes {
			name := base + suffix
			path := filepath.Join(dir, name)
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("sqlstore inspect permissions %s: %w", path, err)
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("sqlstore refusing non-regular SQLite file %s", path)
			}
			if info.Mode().Perm() == 0o600 {
				continue
			}
			if err := os.Chmod(path, 0o600); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("sqlstore chmod %s: %w", path, err)
			}
			repaired = append(repaired, name)
		}
	}
	return repaired, nil
}

func openSQLite(path, params string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?"+params)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// WithSpan은 root 하나의 read-modify-write span을 직렬화하고, 순서 있는
// active-root chain을 전파하며, 로컬 대기와 SQLite lock 대기 모두 ctx를
// 따르게 한다.
func (d *DB) WithSpan(ctx context.Context, fn func(context.Context) error) error {
	return d.withSpan(ctx, time.Now, fn)
}

func (d *DB) withSpan(ctx context.Context, now func() time.Time, fn func(context.Context) error) (err error) {
	if ctx == nil {
		return fmt.Errorf("sqlstore span context is required")
	}
	if fn == nil {
		return fmt.Errorf("sqlstore span callback is required")
	}
	observer := spanObserver(ctx)
	parent, _ := ctx.Value(spanRunKey{}).(*spanRun)
	run := &spanRun{db: d, parent: parent, now: now}
	started := now()
	returned := false
	defer func() {
		ended := run.gateReleased
		if ended.IsZero() {
			ended = now()
		}
		outcome := spanOutcome(err)
		if !returned {
			outcome = SpanOutcomeError
		}
		if observer != nil {
			observer(run.observation(started, ended, outcome))
		}
	}()
	err = d.runSpan(ctx, run, fn)
	returned = true
	return err
}

func spanOutcome(err error) string {
	switch {
	case err == nil:
		return SpanOutcomeSuccess
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return SpanOutcomeCanceled
	case isNestedSpanError(err):
		return SpanOutcomeNested
	default:
		return SpanOutcomeError
	}
}

func (d *DB) runSpan(ctx context.Context, run *spanRun, fn func(context.Context) error) error {
	chain, _ := ctx.Value(spanChainKey{}).([]string)
	chain = append([]string(nil), chain...)
	for _, active := range chain {
		if active == d.dir {
			return &NestedSpanError{
				ActiveDirs:   append([]string(nil), chain...),
				RequestedDir: d.dir,
			}
		}
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-d.spanGate:
	default:
		run.contended = true
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.spanGate:
		}
	}
	defer func() {
		d.spanGate <- struct{}{}
		run.gateReleased = run.now()
	}()
	run.unattributedBase = d.unattributedEpoch.Load()
	inFlightAtBaseline := d.unattributedInFlight.Load() != 0
	tx, sqliteContended, err := d.beginSpanTx(ctx)
	run.contended = run.contended || sqliteContended
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("sqlstore span lock %s: %w", d.dir, ctxErr)
		}
		return fmt.Errorf("sqlstore span lock %s: %w", d.dir, err)
	}
	run.lockAcquired = run.now()
	run.acquired = true
	run.active.Store(true)
	defer func() {
		run.active.Store(false)
		_ = tx.Rollback()
		run.lockReleased = run.now()
		run.unattributedWrites = inFlightAtBaseline || d.unattributedEpoch.Load() != run.unattributedBase
	}()
	spanCtx := context.WithValue(ctx, spanChainKey{}, append(chain, d.dir))
	spanCtx = context.WithValue(spanCtx, spanRunKey{}, run)
	spanCtx, err = d.bindRecordGuard(spanCtx, run)
	if err != nil {
		return err
	}
	return run.invoke(spanCtx, fn)
}

// commitData는 commit 직전에 취소를 확인하고, 취소됐으면 Commit을 부르지 않는다.
// ctx가 이 핸들의 활성 span에서 왔으면 Commit 호출 시간을 그 span에 누적하고,
// 아니면 귀속 불가 write로 센다. 실패한 Commit 호출도 실제 호출이므로 센다.
func (d *DB) commitData(ctx context.Context, tx *sql.Tx) error {
	if d.hooks.beforeDataCommit != nil {
		d.hooks.beforeDataCommit()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	run := activeSpanRun(ctx, d)
	if run == nil {
		return d.unattributedWrite(tx.Commit)
	}
	started := run.now()
	err := tx.Commit()
	run.commitNanos.Add(int64(run.now().Sub(started)))
	run.commitCount.Add(1)
	return err
}

// unattributedWrite는 write 전체를 in-flight로 표시하고 시작과 끝에 epoch를 올린다.
func (d *DB) unattributedWrite(write func() error) error {
	d.unattributedInFlight.Add(1)
	d.unattributedEpoch.Add(1)
	defer func() {
		d.unattributedEpoch.Add(1)
		d.unattributedInFlight.Add(-1)
	}()
	if d.hooks.unattributedWriteStarted != nil {
		d.hooks.unattributedWriteStarted()
	}
	err := write()
	if d.hooks.unattributedWriteFinished != nil {
		d.hooks.unattributedWriteFinished()
	}
	return err
}

func isNestedSpanError(err error) bool {
	_, ok := errors.AsType[*NestedSpanError](err)
	return ok
}

func (d *DB) beginSpanTx(ctx context.Context) (*sql.Tx, bool, error) {
	return d.beginSpanTxAfterContention(ctx, nil)
}

func (d *DB) beginSpanTxAfterContention(
	ctx context.Context,
	afterFirstContention func(),
) (*sql.Tx, bool, error) {
	maxWait := time.NewTimer(spanLockMaxWait)
	defer maxWait.Stop()
	retry := time.NewTimer(time.Hour)
	if !retry.Stop() {
		<-retry.C
	}
	defer retry.Stop()

	contended := false
	retryGap := spanLockInitialRetryGap
	for {
		if err := ctx.Err(); err != nil {
			return nil, contended, err
		}
		// Cancellation stops acquisition and callback work, but must not
		// release an acquired lock while its callback still owns side effects.
		tx, err := d.span.BeginTx(context.WithoutCancel(ctx), nil)
		if err == nil {
			if err := ctx.Err(); err != nil {
				_ = tx.Rollback()
				return nil, contended, err
			}
			return tx, contended, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, contended, ctxErr
		}
		if !isSQLiteLockContention(err) {
			return nil, contended, err
		}
		if !contended {
			contended = true
			if afterFirstContention != nil {
				afterFirstContention()
			}
		}
		retry.Reset(retryGap)
		select {
		case <-ctx.Done():
			return nil, contended, ctx.Err()
		case <-maxWait.C:
			return nil, contended, err
		case <-retry.C:
			retryGap = nextSpanRetryGap(retryGap)
		}
	}
}

func nextSpanRetryGap(current time.Duration) time.Duration {
	return min(current*2, spanLockMaxRetryGap)
}

func isSQLiteLockContention(err error) bool {
	sqliteErr, ok := errors.AsType[*sqlite.Error](err)
	if !ok {
		return false
	}
	primaryCode := sqliteErr.Code() & 0xff
	return primaryCode == int(sqlite3.SQLITE_BUSY) || primaryCode == int(sqlite3.SQLITE_LOCKED)
}

// Get은 (bucket, id)의 record 데이터와 존재 여부를 반환한다.
func (d *DB) Get(bucket, id string) ([]byte, bool, error) {
	var data []byte
	err := d.data.QueryRow(`SELECT data FROM records WHERE bucket = ? AND id = ?`, bucket, id).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

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
	if err == sql.ErrNoRows {
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
func WalkExisting(ctx context.Context, dir, bucket string, visit func(port.RecordRow) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := openExistingData(dir)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, data.Close()) }()
	rows, err := data.QueryContext(ctx, `SELECT id, data FROM records WHERE bucket = ? ORDER BY id`, bucket)
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
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("sqlstore existing data db %s: %w", abs, fs.ErrNotExist)
		}
		return nil, err
	}
	return openSQLite(dataPath, fmt.Sprintf("mode=ro&_pragma=busy_timeout(%d)&_pragma=query_only(1)", existingReadBusyTimeout/time.Millisecond))
}

// Put은 (bucket, id)의 record 데이터를 upsert한다.
func (d *DB) Put(bucket, id string, data []byte) error {
	release, err := d.acquireRecordWriter(context.Background())
	if err != nil {
		return err
	}
	defer release()
	return d.unattributedWrite(func() error {
		_, err := d.data.Exec(`INSERT INTO records (bucket, id, data) VALUES (?, ?, ?)
		ON CONFLICT (bucket, id) DO UPDATE SET data = excluded.data`, bucket, id, data)
		return err
	})
}

func (d *DB) Mutate(ctx context.Context, mutations []stateport.Mutation) error {
	converted := make([]port.RecordMutation, 0, len(mutations))
	for _, mutation := range mutations {
		converted = append(converted, port.RecordMutation{
			Bucket:        mutation.Bucket,
			ID:            mutation.ID,
			Data:          mutation.Data,
			Delete:        mutation.Delete,
			RequireAbsent: mutation.RequireAbsent,
		})
	}
	return d.Apply(ctx, converted)
}

// Apply는 모든 mutation을 issueops.db 트랜잭션 하나로 commit한다.
func (d *DB) Apply(ctx context.Context, mutations []port.RecordMutation) error {
	if len(mutations) == 0 {
		return nil
	}
	if err := validateMutations(mutations); err != nil {
		return err
	}
	release, err := d.acquireRecordWriter(ctx)
	if err != nil {
		return err
	}
	defer release()
	tx, err := d.data.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := applyMutationsTx(ctx, tx, mutations); err != nil {
		return err
	}
	if err := d.recheckRecordGuard(ctx, tx); err != nil {
		return err
	}
	return d.commitData(ctx, tx)
}

// CompareAndApply는 raw-byte 비교와 write를 하나의 data.sqlite transaction으로
// 묶는다. 권한 CAS는 앞선 Get으로 비교를 분리하면 안 된다.
func (d *DB) CompareAndApply(ctx context.Context, expected []port.ExpectedRecord, mutations []port.RecordMutation) error {
	if len(mutations) == 0 {
		return nil
	}
	if err := validateMutations(mutations); err != nil {
		return err
	}
	return d.CompareAndApplyFunc(ctx, expected, func() ([]port.RecordMutation, error) { return mutations, nil })
}

// CompareAndApplyFunc는 raw CAS가 성공한 뒤에만 encoder가 mutation을 만들게 해,
// stale snapshot에서 만든 payload가 transaction 밖으로 새지 않게 한다.
func (d *DB) CompareAndApplyFunc(ctx context.Context, expected []port.ExpectedRecord, build func() ([]port.RecordMutation, error)) error {
	if build == nil {
		return fmt.Errorf("sqlstore compare-and-apply mutation builder is required")
	}
	for _, item := range expected {
		if item.Bucket == "" || item.ID == "" || item.Data == nil {
			return fmt.Errorf("sqlstore expected record bucket, id, and data are required")
		}
	}
	release, err := d.acquireRecordWriter(ctx)
	if err != nil {
		return err
	}
	defer release()
	tx, err := d.data.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, item := range expected {
		var current []byte
		err := tx.QueryRowContext(ctx, `SELECT data FROM records WHERE bucket = ? AND id = ?`, item.Bucket, item.ID).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !bytes.Equal(current, item.Data)) {
			return &RawCASError{Bucket: item.Bucket, ID: item.ID}
		}
		if err != nil {
			return err
		}
	}
	mutations, err := build()
	if err != nil {
		return err
	}
	if len(mutations) == 0 {
		return nil
	}
	if err := validateMutations(mutations); err != nil {
		return err
	}
	if err := applyMutationsTx(ctx, tx, mutations); err != nil {
		return err
	}
	if err := d.recheckRecordGuard(ctx, tx); err != nil {
		return err
	}
	return d.commitData(ctx, tx)
}

func validateMutations(mutations []port.RecordMutation) error {
	for _, mutation := range mutations {
		if mutation.Delete && mutation.RequireAbsent {
			return fmt.Errorf("sqlstore delete mutation cannot require an absent row")
		}
		if mutation.Bucket == "" || mutation.ID == "" {
			return fmt.Errorf("sqlstore mutation bucket and id are required")
		}
	}
	return nil
}

func applyMutationsTx(ctx context.Context, tx *sql.Tx, mutations []port.RecordMutation) error {
	for _, mutation := range mutations {
		if mutation.RequireAbsent {
			var present int
			err := tx.QueryRowContext(ctx, `SELECT 1 FROM records WHERE bucket = ? AND id = ?`, mutation.Bucket, mutation.ID).Scan(&present)
			switch {
			case err == nil:
				return fmt.Errorf("sqlstore precondition failed: row %s/%s already exists", mutation.Bucket, mutation.ID)
			case !errors.Is(err, sql.ErrNoRows):
				return err
			}
		}
		if mutation.Delete {
			if _, err := tx.ExecContext(ctx, `DELETE FROM records WHERE bucket = ? AND id = ?`, mutation.Bucket, mutation.ID); err != nil {
				return err
			}
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO records (bucket, id, data) VALUES (?, ?, ?)
			ON CONFLICT (bucket, id) DO UPDATE SET data = excluded.data`, mutation.Bucket, mutation.ID, mutation.Data); err != nil {
			return err
		}
	}
	return nil
}

// Delete는 (bucket, id)의 record를 제거한다. 없는 record를 삭제해도 오류가
// 아니다.
func (d *DB) Delete(bucket, id string) error {
	release, err := d.acquireRecordWriter(context.Background())
	if err != nil {
		return err
	}
	defer release()
	return d.unattributedWrite(func() error {
		_, err := d.data.Exec(`DELETE FROM records WHERE bucket = ? AND id = ?`, bucket, id)
		return err
	})
}

// List는 bucket의 id를 오름차순으로 반환한다.
func (d *DB) List(bucket string) ([]string, error) {
	rows, err := d.data.Query(`SELECT id FROM records WHERE bucket = ? ORDER BY id`, bucket)
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

// GetAll은 bucket의 모든 record를 id 순으로 반환한다.
func (d *DB) GetAll(bucket string) ([]port.RecordRow, error) {
	rows, err := d.data.Query(`SELECT id, data FROM records WHERE bucket = ? ORDER BY id`, bucket)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []port.RecordRow{}
	for rows.Next() {
		var r port.RecordRow
		if err := rows.Scan(&r.ID, &r.Data); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteBucket은 bucket의 모든 record를 제거한다.
func (d *DB) DeleteBucket(bucket string) error {
	release, err := d.acquireRecordWriter(context.Background())
	if err != nil {
		return err
	}
	defer release()
	return d.unattributedWrite(func() error {
		_, err := d.data.Exec(`DELETE FROM records WHERE bucket = ?`, bucket)
		return err
	})
}
