package state

import (
	"context"
	"fmt"
	"time"

	statecontract "issueops/internal/contract/state"
	statedomain "issueops/internal/domain/state"
	stateport "issueops/internal/port/state"
)

// Prune removes (or, without confirm, only reports) every state record whose
// UpdatedAt is older than maxAge. maxAge must be positive.
func (service *Service) Prune(ctx context.Context, maxAge time.Duration, confirm bool) (statecontract.PruneResult, error) {
	return service.pruneMatching(ctx, "", maxAge, 0, confirm)
}

// PrunePrefix is Prune restricted to keys that start with prefix; prefix is
// required. A positive maxRecords also applies the retained-count policy of
// statedomain.SelectPrune. Without confirm it is a dry run.
func (service *Service) PrunePrefix(ctx context.Context, prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.PruneResult, error) {
	if prefix == "" {
		return service.newPruneResult(maxAge, confirm), fmt.Errorf("prefix is required")
	}
	return service.pruneMatching(ctx, prefix, maxAge, maxRecords, confirm)
}

func (service *Service) pruneMatching(ctx context.Context, prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.PruneResult, error) {
	result := service.newPruneResult(maxAge, confirm)
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive: pass a Go duration such as 720h (CLI --max-age, MCP max_age)")
	}
	if maxRecords < 0 {
		return result, fmt.Errorf("max records must be non-negative")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	cutoff := service.now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	prune := func(spanCtx context.Context, store stateport.Store) error {
		list, err := service.List()
		if err != nil {
			return err
		}
		result.Pruned, result.Kept = statedomain.SelectPrune(list.Records, prefix, cutoff, maxRecords)
		for _, record := range result.Pruned {
			result.DeletedKeys = append(result.DeletedKeys, record.Key)
			if confirm {
				if err := store.Mutate(spanCtx, []stateport.Mutation{{Bucket: stateBucket, ID: record.Key, Delete: true}}); err != nil {
					return err
				}
			}
		}
		for _, record := range result.Kept {
			result.KeptKeys = append(result.KeptKeys, record.Key)
		}
		result.OK = true
		return nil
	}
	if !confirm {
		err := prune(ctx, nil)
		return result, err
	}
	store, err := service.dependencies.OpenStore(result.StateDir)
	if err != nil {
		return result, err
	}
	err = store.WithSpan(ctx, func(spanCtx context.Context) error { return prune(spanCtx, store) })
	return result, err
}

func (service *Service) newPruneResult(maxAge time.Duration, confirm bool) statecontract.PruneResult {
	return statecontract.PruneResult{
		OK:          false,
		StateDir:    service.stateDir(),
		MaxAge:      maxAge.String(),
		Confirm:     confirm,
		DryRun:      !confirm,
		DeletedKeys: []string{},
		Pruned:      []statecontract.ListEntry{},
		KeptKeys:    []string{},
		Kept:        []statecontract.ListEntry{},
	}
}
