package sqlstore

import (
	"errors"
	"fmt"
	"path/filepath"
)

// closeRoot는 dir의 캐시된 핸들을 닫고 축출한다. 캐시되지 않은 root를 닫는
// 것은 no-op이다.
func closeRoot(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("sqlstore close %q: %w", dir, err)
	}
	handlesMu.Lock()
	db, ok := handles[abs]
	if ok {
		delete(handles, abs)
	}
	handlesMu.Unlock()
	if !ok {
		return nil
	}
	return errors.Join(db.data.Close(), db.span.Close())
}
