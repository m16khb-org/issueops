package issueopslease

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"issueops/internal/port"
)

type reseedFenceStore func(string) (port.TransactionalRecordStore, error)

// SQLiteReseedFence 타입은 lifecycle ID마다 별도 SQLite 저장소를 열어 같은 ID에 대한 reseed를 직렬화한다.
type SQLiteReseedFence struct {
	root string
	open reseedFenceStore
}

// NewSQLiteReseedFence 함수는 stateRoot 아래에 fence 저장소를 여는 SQLiteReseedFence를 만든다.
// stateRoot를 절대 경로로 만들 수 없거나 open이 nil이면 오류를 반환한다.
func NewSQLiteReseedFence(stateRoot string, open reseedFenceStore) (*SQLiteReseedFence, error) {
	root, err := filepath.Abs(stateRoot)
	if err != nil {
		return nil, err
	}
	if open == nil {
		return nil, fmt.Errorf("reseed fence store opener is required")
	}
	return &SQLiteReseedFence{root: root, open: open}, nil
}

// Within 메서드는 id에 대응하는 fence 저장소를 열고 그 span 안에서 fn을 실행한다. 저장소를 열 수 없으면 오류를 반환한다.
func (f *SQLiteReseedFence) Within(ctx context.Context, id string, fn func(context.Context) error) error {
	sum := sha256.Sum256([]byte(id))
	db, err := f.open(filepath.Join(f.root, "issueops-reseed-fence", hex.EncodeToString(sum[:])))
	if err != nil {
		return err
	}
	return db.WithSpan(ctx, fn)
}
