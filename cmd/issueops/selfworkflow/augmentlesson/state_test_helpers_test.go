package augmentlesson

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
	"time"
)

// production wiring과 같은 state store를 설치한다. fitness graph는 test import를
// 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	StateDir = statestore.StateDir
	StatePrunePrefix = func(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
		return statestore.StatePrunePrefix(context.Background(), prefix, maxAge, maxRecords, confirm)
	}
	StateWrite = func(key, content string) (statecontract.StateResult, error) {
		return statestore.StateWrite(context.Background(), key, content)
	}
}
