package issueopsinventory

import (
	"context"
	"fmt"
	"time"

	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsinventorydomain "issueops/internal/domain/issueopsinventory"
)

type Service struct {
	repository Repository
	clock      Clock
	paths      PathNormalizer
}

func NewService(repository Repository, clock Clock, paths PathNormalizer) *Service {
	return &Service{repository: repository, clock: clock, paths: paths}
}

func (service *Service) ListCycles(
	ctx context.Context,
	stateRoot string,
	repo string,
) (issueopsinventorycontract.ListResult, error) {
	if service == nil || service.repository == nil || service.clock == nil || service.paths == nil {
		return issueopsinventorycontract.ListResult{OK: false}, fmt.Errorf("issueops inventory dependencies are required")
	}
	if err := ctx.Err(); err != nil {
		return issueopsinventorycontract.ListResult{OK: false}, err
	}
	rawRepo := repo
	repo = service.paths.Normalize(repo)
	normalizedRecordPaths := map[string]string{rawRepo: repo}
	entries := []issueopsinventorycontract.ListEntry{}
	scannedRecords := 0
	diagnostics, err := service.repository.ScanEach(ctx, stateRoot, func(record issueopsinventorycontract.Record) error {
		scannedRecords++
		if repo != "" {
			normalizedRepo, ok := normalizedRecordPaths[record.Repo]
			if !ok {
				normalizedRepo = service.paths.Normalize(record.Repo)
				normalizedRecordPaths[record.Repo] = normalizedRepo
			}
			if normalizedRepo != repo {
				return nil
			}
		}
		entries = append(entries, issueopsinventorydomain.ProjectEntry(record))
		return nil
	})
	if err != nil {
		return issueopsinventorycontract.ListResult{OK: false}, err
	}
	result := issueopsinventorycontract.ListResult{
		OK:             true,
		GeneratedAt:    service.clock.Now().UTC().Format(time.RFC3339),
		ScannedRecords: scannedRecords + len(diagnostics),
		ReadErrors:     len(diagnostics),
		UnreadableIDs:  []string{},
		Diagnostics:    diagnostics,
		Entries:        entries,
	}
	for _, diagnostic := range diagnostics {
		result.UnreadableIDs = append(result.UnreadableIDs, diagnostic.ID)
	}
	return result, nil
}
