package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func mutateReviewRecord(store reviewport.ReviewMutationStore, stateRoot, id string, prepare func() error, apply func(*model.Record) error) (model.Record, error) {
	var result model.Record
	err := store.WithLock(stateRoot, id, func() error {
		current, err := store.Read(stateRoot, id)
		if err != nil {
			return err
		}
		if err := store.ValidateMutation(current); err != nil {
			return err
		}
		if prepare != nil {
			if err := prepare(); err != nil {
				return err
			}
		}
		current, err = store.Read(stateRoot, id)
		if err != nil {
			return err
		}
		if err := apply(&current); err != nil {
			return err
		}
		result, err = store.Write(stateRoot, current)
		return err
	})
	if err != nil {
		return model.Record{OK: false}, err
	}
	return result, nil
}
