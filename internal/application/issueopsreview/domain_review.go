package issueopsreview

import (
	"strings"

	model "issueops/internal/contract/issueops"
	issueopsintent "issueops/internal/domain/issueopsintent"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func RecordDomainReview(store reviewport.ReviewMutationStore, stateRoot, id string, req model.IssueOpsDomainReviewRequest) (model.IssueOpsRecord, error) {
	var modelFit string
	var terminology []string
	return mutateReviewRecord(store, stateRoot, id, func() error {
		modelFit = strings.TrimSpace(req.ModelFit)
		terminology = issueopsintent.CleanTextValues(req.Terminology)
		return reviewdomain.ValidateDomainReviewRecord(modelFit, len(terminology))
	}, func(record *model.IssueOpsRecord) error {
		now := store.Now()
		record.DomainReview = &model.IssueOpsDomainReview{
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
