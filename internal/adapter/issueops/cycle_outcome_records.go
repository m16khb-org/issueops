package issueops

import (
	"errors"
	"io/fs"

	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// cycleOutcomeBucket keeps one metrics summary per deleted cycle record. Cleanup
// and prune write it in the same transaction that deletes the record, so the
// summary exists exactly when the record is gone.
const cycleOutcomeBucket = port.CycleOutcomeBucket

func cycleOutcomeMutation(recordID string, outcome model.CycleOutcome) (port.RecordMutation, error) {
	data, err := model.EncodeCycleOutcome(recordID, outcome)
	if err != nil {
		return port.RecordMutation{}, err
	}
	return port.RecordMutation{Bucket: cycleOutcomeBucket, ID: recordID, Data: data}, nil
}

// ListCycleOutcomes reads every retained cycle outcome. Rows that fail to decode
// or carry another schema are returned as unreadable IDs instead of guessed.
func ListCycleOutcomes(stateRoot string) ([]model.CycleOutcome, []string, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, nil, err
	}
	rows, err := db.GetAll(cycleOutcomeBucket)
	if err != nil {
		return nil, nil, err
	}
	outcomes := make([]model.CycleOutcome, 0, len(rows))
	var unreadable []string
	for _, row := range rows {
		outcome, err := model.DecodeCycleOutcome(row.ID, row.Data)
		if err != nil {
			unreadable = append(unreadable, row.ID)
			continue
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, unreadable, nil
}

// ReadCycleOutcome reads the retained outcome of one deleted cycle without
// creating, repairing, or migrating the state store. A missing store and a
// missing row both answer (zero, false, nil); a row that fails to decode, carries
// another schema, or names another cycle is an error, not a guess.
func ReadCycleOutcome(stateRoot, id string) (model.CycleOutcome, bool, error) {
	data, ok, err := sqlstore.GetExisting(stateRoot, cycleOutcomeBucket, id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return model.CycleOutcome{}, false, nil
		}
		return model.CycleOutcome{}, false, err
	}
	if !ok {
		return model.CycleOutcome{}, false, nil
	}
	outcome, err := model.DecodeCycleOutcome(id, data)
	if err != nil {
		return model.CycleOutcome{}, false, err
	}
	return outcome, true, nil
}
