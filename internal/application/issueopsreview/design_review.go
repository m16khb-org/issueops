package issueopsreview

import (
	"fmt"
	reviewcontract "issueops/internal/contract/issueopsreview"
	"strings"

	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func RecordDesignReview(store reviewport.DesignReviewStore, stateRoot, id string, req reviewcontract.DesignReviewRequest) (model.IssueOpsRecord, error) {
	review, err := reviewdomain.PrepareDesignReview(req)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := store.PlanReadiness(record); !ready.Ready {
		if blocking := reviewdomain.NonPlanPrepMissing(ready.Missing); len(blocking) > 0 {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot record design review before intent contract: missing %s", strings.Join(blocking, ", "))
		}
	}
	review, err = reviewdomain.FinalizeDesignReview(review, store.Now())
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record.DesignReview = &review
	return store.TouchWrite(stateRoot, record)
}
