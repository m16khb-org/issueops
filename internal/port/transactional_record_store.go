package port

import "context"

// TransactionalRecordStore 타입은 release repository가 proven SQLite store에 요구하는 최소 원자 연산만 노출한다. concrete
// store 선택은 composition root가 소유한다.
type TransactionalRecordStore interface {
	WithSpan(context.Context, func(context.Context) error) error
	Get(string, string) ([]byte, bool, error)
	Apply(context.Context, []RecordMutation) error
}

// RecordInventoryStore extends the transactional record surface only for
// capabilities that must fail closed while scanning a complete bucket.
type RecordInventoryStore interface {
	TransactionalRecordStore
	GetAll(string) ([]RecordRow, error)
}

// RawCASFailure is implemented by a store error raised when a compare-and-swap precondition failed;
// FailedBucket names the bucket whose record no longer matched.
type RawCASFailure interface {
	error
	FailedBucket() string
}

// RecordRow is one stored record as returned by a full-bucket scan.
type RecordRow struct {
	ID   string
	Data []byte
}

// ExpectedRecord is the precondition of a compare-and-apply: the record at Bucket and ID must still
// hold exactly Data.
type ExpectedRecord struct {
	Bucket string
	ID     string
	Data   []byte
}

// RecordMutation is one write in an atomic Apply: it stores Data at Bucket and ID, or deletes the
// record when Delete is set. RequireAbsent makes the write fail if the record already exists.
type RecordMutation struct {
	Bucket        string
	ID            string
	Data          []byte
	Delete        bool
	RequireAbsent bool
}
