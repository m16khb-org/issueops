package state

import (
	"context"
	"time"

	statecontract "issueops/internal/contract/state"
)

func StatePrune(ctx context.Context, maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error) {
	return service().Prune(ctx, maxAge, confirm)
}

func StatePrunePrefix(ctx context.Context, prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
	return service().PrunePrefix(ctx, prefix, maxAge, maxRecords, confirm)
}
