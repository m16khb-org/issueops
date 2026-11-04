package issueopsreview

import (
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsintent "issueops/internal/domain/issueopsintent"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordDomainReview validates req (a model-fit statement and terminology are
// required) and stores it as the domain review of record id, stamping ReviewedAt
// and UpdatedAt under the record lock.
func RecordDomainReview(store reviewport.ReviewMutationStore, stateRoot, id string, req model.DomainReviewRequest) (model.Record, error) {
	var modelFit string
	var terminology []string
	return mutateReviewRecord(store, stateRoot, id, func() error {
		modelFit = strings.TrimSpace(req.ModelFit)
		terminology = issueopsintent.CleanTextValues(req.Terminology)
		return reviewdomain.ValidateDomainReviewRecord(modelFit, len(terminology))
	}, func(record *model.Record) error {
		now := store.Now()
		record.DomainReview = &model.DomainReview{
			Terminology:       terminology,
			ModelFit:          modelFit,
			Risks:             issueopsintent.CleanTextValues(req.Risks),
			OpenUncertainties: issueopsintent.CleanTextValues(req.OpenUncertainties),
			ReviewedAt:        now,
		}
		record.UpdatedAt = now
		return nil
	})
}
