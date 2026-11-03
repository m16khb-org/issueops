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
	matchesRepo := service.repoFilter(repo)
	entries := []issueopsinventorycontract.ListEntry{}
	scannedRecords := 0
	diagnostics, err := service.repository.ScanEach(ctx, stateRoot, func(record issueopsinventorycontract.Record) error {
		scannedRecords++
		if !matchesRepo(record.Repo) {
			return nil
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

// repoFilter returns a predicate reporting whether a record's repo is the requested repo; an empty request matches every record.
func (service *Service) repoFilter(rawRepo string) func(recordRepo string) bool {
	repo := service.paths.Normalize(rawRepo)
	normalizedRecordPaths := map[string]string{rawRepo: repo}
	// Lexical identities that differ may still name one directory through a
	// symlink (/tmp vs /private/tmp on macOS); resolve each once.
	canonical := map[string]string{}
	canonicalOf := func(path string) string {
		resolved, ok := canonical[path]
		if !ok {
			resolved = service.paths.Canonical(path)
			canonical[path] = resolved
		}
		return resolved
	}
	return func(recordRepo string) bool {
		if repo == "" {
			return true
		}
		normalizedRepo, ok := normalizedRecordPaths[recordRepo]
		if !ok {
			normalizedRepo = service.paths.Normalize(recordRepo)
			normalizedRecordPaths[recordRepo] = normalizedRepo
		}
		return normalizedRepo == repo || canonicalOf(normalizedRepo) == canonicalOf(repo)
	}
}
