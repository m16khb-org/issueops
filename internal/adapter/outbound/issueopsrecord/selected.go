package issueopsrecord

import (
	"context"
	"errors"
	"io/fs"

	issueopscontract "issueops/internal/contract/issueops"
	statecontract "issueops/internal/contract/state"
)

// ReadSelected preserves selection semantics without enumerating sibling rows.
func (store Store) ReadSelected(ctx context.Context, stateRoot, id string) (issueopscontract.IssueOpsRecord, error) {
	if err := ctx.Err(); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	invalid := issueopscontract.IssueOpsRecord{ID: id, Invalid: true}
	if _, err := NormalizeID(id); err != nil {
		return invalid, nil
	}
	record, err := store.Read(ctx, stateRoot, id)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, statecontract.ErrInvalidState) {
		return invalid, nil
	}
	return record, err
}
