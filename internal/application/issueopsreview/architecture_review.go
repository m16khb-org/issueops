package issueopsreview

import (
	"fmt"
	reviewcontract "issueops/internal/contract/issueopsreview"
	"strings"

	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordArchitectureReview validates req, binds it to the digest of the current
// architecture document and stores it on record id. It fails when the intent
// contract is not yet recorded or the architecture document resolver is missing.
func RecordArchitectureReview(store reviewport.ArchitectureReviewStore, stateRoot, id string, req reviewcontract.ArchitectureReviewRequest) (model.Record, error) {
	review, err := reviewdomain.PrepareArchitectureReview(req)
	if err != nil {
		return model.Record{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := store.PlanReadiness(record); !ready.Ready {
		if blocking := reviewdomain.NonPlanPrepMissing(ready.Missing); len(blocking) > 0 {
			return model.Record{OK: false}, fmt.Errorf("cannot record architecture review before intent contract: missing %s", strings.Join(blocking, ", "))
		}
	}
	review, err = reviewdomain.FinalizeArchitectureReview(review, store.Now())
	if err != nil {
		return model.Record{OK: false}, err
	}
	if store.Document == nil {
		return model.Record{OK: false}, fmt.Errorf("architecture document resolver is unavailable")
	}
	body, err := store.Document(stateRoot, record)
	if err != nil {
		return model.Record{OK: false}, err
	}
	digest, err := reviewdomain.ArchitectureDocumentDigest(body)
	if err != nil {
		return model.Record{OK: false}, err
	}
	review.ReviewedArchitectureDigest = digest
	review = reviewdomain.ApplyArchitectureReview(record.ArchitectureReview, review)
	record.ArchitectureReview = &review
	return store.TouchWrite(stateRoot, record)
}
