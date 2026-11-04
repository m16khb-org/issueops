package issueopsretention

import (
	"context"
	"errors"
	"fmt"
	"time"

	issueopsretentioncontract "issueops/internal/contract/issueopsretention"
	issueopsretentiondomain "issueops/internal/domain/issueopsretention"
)

// Service prunes expired issueops records through a Repository, using an
// injected clock to compute the cutoff.
type Service struct {
	repository Repository
	clock      clock
}

const unreadableReportLimit = 20

var errIncomplete = errors.New("issueops retention incomplete")

// NewService returns a Service that reads and deletes records through
// repository and takes the current time from clock.
func NewService(repository Repository, clock clock) *Service {
	return &Service{repository: repository, clock: clock}
}

// Prune deletes records in stateRoot last updated before now minus maxAge.
// Without confirm it only reports what would be pruned (dry run). Unreadable
// records and delete failures make the run incomplete and return an error
// wrapping the incomplete sentinel alongside the partial result.
func (service *Service) Prune(
	ctx context.Context,
	stateRoot string,
	maxAge time.Duration,
	confirm bool,
) (issueopsretentioncontract.Result, error) {
	result := issueopsretentioncontract.Result{
		StateRoot:             stateRoot,
		MaxAge:                maxAge.String(),
		DryRun:                !confirm,
		Pruned:                []string{},
		Kept:                  []string{},
		Unreadable:            []string{},
		UnreadableDiagnostics: []issueopsretentioncontract.UnreadableRecordDiagnostic{},
		Failed:                []string{},
		DeleteDiagnostics:     []issueopsretentioncontract.DeleteFailureDiagnostic{},
	}
	if service == nil || service.repository == nil || service.clock == nil {
		return result, fmt.Errorf("issueops retention dependencies are required")
	}
	if maxAge <= 0 {
		return result, fmt.Errorf("max age must be positive")
	}
	cutoff := service.clock.Now().UTC().Add(-maxAge)
	result.Cutoff = cutoff.Format(time.RFC3339Nano)
	ids, err := service.repository.ListIDs(ctx, stateRoot)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		record, err := service.repository.ReadUnchecked(ctx, stateRoot, id)
		if err != nil {
			noteUnreadable(&result, id)
			continue
		}
		if !issueopsretentiondomain.IsPrunable(record, cutoff) {
			result.Kept = append(result.Kept, id)
			continue
		}
		if confirm {
			if err := service.repository.DeleteIfUnchanged(ctx, stateRoot, id, record); err != nil {
				noteDeleteFailure(&result, id)
				result.Error = errIncomplete.Error()
				return result, fmt.Errorf("%w: delete %s: %v", errIncomplete, id, err) //nolint:errorlint // callers classify on ErrIncomplete alone; the delete failure is diagnostic text
			}
		}
		result.Pruned = append(result.Pruned, id)
	}
	if result.ReadErrors > 0 {
		result.Error = errIncomplete.Error()
		return result, fmt.Errorf("%w: %d unreadable record(s)", errIncomplete, result.ReadErrors)
	}
	result.OK = true
	return result, nil
}

func noteUnreadable(result *issueopsretentioncontract.Result, id string) {
	result.ReadErrors++
	if len(result.Unreadable) < unreadableReportLimit {
		result.Unreadable = append(result.Unreadable, id)
		result.UnreadableDiagnostics = append(result.UnreadableDiagnostics, issueopsretentioncontract.UnreadableRecordDiagnostic{
			ID:   id,
			Code: "read_failed",
		})
	}
}

func noteDeleteFailure(result *issueopsretentioncontract.Result, id string) {
	result.DeleteErrors++
	if len(result.Failed) < unreadableReportLimit {
		result.Failed = append(result.Failed, id)
		result.DeleteDiagnostics = append(
			result.DeleteDiagnostics,
			issueopsretentioncontract.DeleteFailureDiagnostic{ID: id, Code: "delete_failed"},
		)
	}
}
