package issueopsreview

import (
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	issueopsintent "issueops/internal/domain/issueopsintent"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func RecordAISlopCleanEvidence(store reviewport.AISlopCleanStore, stateRoot, id string, categories, verification []string) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
	err := store.WithLock(stateRoot, id, func() error {
		current, err := store.Read(stateRoot, id)
		if err != nil {
			return err
		}
		if err := store.ValidateMutation(current); err != nil {
			return err
		}
		cleanCategories := issueopsintent.CleanTextValues(categories)
		cleanVerification := issueopsintent.CleanTextValues(verification)
		if err := reviewdomain.ValidateAISlopCleanEvidence(cleanCategories, cleanVerification); err != nil {
			return err
		}
		current, err = store.Read(stateRoot, id)
		if err != nil {
			result = current
			return err
		}
		current.AISlopCleanCategories = cleanCategories
		current.AISlopCleanVerification = cleanVerification
		if strings.TrimSpace(current.AISlopCleanAt) != "" && issueopsdomain.IssueOpsPhaseRank(current.Phase) >= issueopsdomain.IssueOpsPhaseRank(model.IssueOpsPhaseAISlopClean) {
			result, err = store.Refresh(stateRoot, current)
			return err
		}
		current.UpdatedAt = store.Now()
		result, err = store.Write(stateRoot, current)
		return err
	})
	return result, err
}
