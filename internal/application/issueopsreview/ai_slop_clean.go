package issueopsreview

import (
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	issueopsintent "issueops/internal/domain/issueopsintent"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordAISlopCleanEvidence stores the cleaned categories and verification
// commands of the ai-slop-clean stage on record id under the record lock. The
// write refreshes the record without a phase change when the stage was already
// recorded and the phase is at or past ai-slop-clean; otherwise it stamps UpdatedAt.
func RecordAISlopCleanEvidence(store reviewport.AISlopCleanStore, stateRoot, id string, categories, verification []string) (model.Record, error) {
	var result model.Record
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
		if strings.TrimSpace(current.AISlopCleanAt) != "" && issueopsdomain.PhaseRank(current.Phase) >= issueopsdomain.PhaseRank(model.PhaseAISlopClean) {
			result, err = store.Refresh(stateRoot, current)
			return err
		}
		current.UpdatedAt = store.Now()
		result, err = store.Write(stateRoot, current)
		return err
	})
	return result, err
}
