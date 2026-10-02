package state

import (
	"context"
	"fmt"
	"time"

	statecontract "issueops/internal/contract/state"
	statedomain "issueops/internal/domain/state"
)

func (service *Service) Prune(ctx context.Context, maxAge time.Duration, confirm bool) (statecontract.StatePruneResult, error) {
	return service.prune(ctx, "", maxAge, 0, confirm)
}

func (service *Service) PrunePrefix(ctx context.Context, prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
	if prefix == "" {
		return service.newPruneResult(maxAge, confirm), fmt.Errorf("prefix is required")
	}
	return service.prune(ctx, prefix, maxAge, maxRecords, confirm)
}

func (service *Service) prune(ctx context.Context, prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
	result := service.newPruneResult(maxAge, confirm)
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	if maxRecords < 0 {
		return result, fmt.Errorf("max records must be non-negative")
	}
	cutoff := service.now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	list, err := service.List()
	if err != nil {
		return result, err
	}
	result.Pruned, result.Kept = statedomain.SelectPrune(list.Records, prefix, cutoff, maxRecords)
	for _, record := range result.Pruned {
		result.DeletedKeys = append(result.DeletedKeys, record.Key)
		if confirm {
			if err := service.Delete(ctx, record.Key); err != nil {
				return result, err
			}
		}
	}
	for _, record := range result.Kept {
		result.KeptKeys = append(result.KeptKeys, record.Key)
	}
	result.OK = true
	return result, nil
}

func (service *Service) newPruneResult(maxAge time.Duration, confirm bool) statecontract.StatePruneResult {
	return statecontract.StatePruneResult{
		OK:          false,
		StateDir:    service.stateDir(),
		MaxAge:      maxAge.String(),
		Confirm:     confirm,
		DryRun:      !confirm,
		DeletedKeys: []string{},
		Pruned:      []statecontract.StateListEntry{},
		KeptKeys:    []string{},
		Kept:        []statecontract.StateListEntry{},
	}
}
