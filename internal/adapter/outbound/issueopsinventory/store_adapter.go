package issueopsinventory

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

type Repository struct {
	Store issueopsrecord.Store
}

func (repository Repository) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	return repository.Store.ListIDs(ctx, stateRoot)
}

func (repository Repository) ReadUnchecked(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsinventorycontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}

func (repository Repository) Scan(
	ctx context.Context,
	stateRoot string,
) ([]issueopsinventorycontract.Record, []issueopsinventorycontract.RecordDiagnostic, error) {
	records := []issueopsinventorycontract.Record{}
	diagnostics, err := repository.ScanEach(ctx, stateRoot, func(record issueopsinventorycontract.Record) error {
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return records, diagnostics, nil
}

func (repository Repository) ScanEach(
	ctx context.Context,
	stateRoot string,
	visit func(issueopsinventorycontract.Record) error,
) ([]issueopsinventorycontract.RecordDiagnostic, error) {
	diagnostics, err := repository.Store.ScanEach(ctx, stateRoot, visit)
	if err != nil {
		return nil, err
	}
	result := make([]issueopsinventorycontract.RecordDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, issueopsinventorycontract.RecordDiagnostic{
			ID:   diagnostic.ID,
			Code: diagnostic.Code,
		})
	}
	return result, nil
}
