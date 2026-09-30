package issueopsreview

import (
	model "issueops/internal/contract/issueops"
	reviewport "issueops/internal/port/issueopsreview"
)

func mutateReviewRecord(store reviewport.ReviewMutationStore, stateRoot, id string, prepare func() error, apply func(*model.IssueOpsRecord) error) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
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
		return model.IssueOpsRecord{OK: false}, err
	}
	return result, nil
}
