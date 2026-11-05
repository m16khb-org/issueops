package issueopsinventory

import (
	"context"

	"issueops/internal/adapter/outbound/issueopsrecord"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

// Repository adapts the cycle record store to the inventory contract, converting
// the store's scan diagnostics into inventory diagnostics.
type Repository struct {
	Store issueopsrecord.Store
}

// ListIDs returns the ids of all cycle records under stateRoot.
func (repository Repository) ListIDs(ctx context.Context, stateRoot string) ([]string, error) {
	return repository.Store.ListIDs(ctx, stateRoot)
}

// ReadUnchecked reads the cycle record id under stateRoot exactly as the record
// store returns it.
func (repository Repository) ReadUnchecked(
	ctx context.Context,
	stateRoot string,
	id string,
) (issueopsinventorycontract.Record, error) {
	return repository.Store.Read(ctx, stateRoot, id)
}

// Scan reads every cycle record under stateRoot and returns the readable records
// with a diagnostic for each record it could not use.
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

// ScanEach streams each readable cycle record under stateRoot to visit instead of
// collecting them, and returns a diagnostic for each record it could not use. An
// error from visit or the store stops the scan.
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
