package compatibilityreview

import (
	"time"

	reviewapp "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

type Store = reviewport.CompatibilityStore

func Record(store Store, stateRoot, id string, req model.IssueOpsCompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	return reviewapp.RecordCompatibilityReview(store, stateRoot, id, req, time.Now().UTC().Format(time.RFC3339Nano))
}

func Validate(req model.IssueOpsCompatibilityReviewRequest) (model.IssueOpsCompatibilityReview, error) {
	return reviewdomain.ValidateCompatibilityReview(req, time.Now().UTC().Format(time.RFC3339Nano))
}
