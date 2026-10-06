package compatibilityreview

import (
	reviewcontract "issueops/internal/contract/issueopsreview"
	"time"

	reviewapp "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func Record(store reviewport.CompatibilityStore, stateRoot, id string, req reviewcontract.CompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	return reviewapp.RecordCompatibilityReview(store, stateRoot, id, req, time.Now().UTC().Format(time.RFC3339Nano))
}

func Validate(req reviewcontract.CompatibilityReviewRequest) (reviewcontract.CompatibilityReview, error) {
	return reviewdomain.ValidateCompatibilityReview(req, time.Now().UTC().Format(time.RFC3339Nano))
}
