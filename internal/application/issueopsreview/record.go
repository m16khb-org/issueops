package issueopsreview

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

func RecordDevilsAdvocate(store reviewport.DevilsAdvocateStore, stateRoot, id string, req model.IssueOpsDevilsAdvocateReviewRequest, recordedAt string) (model.IssueOpsRecord, error) {
	review, err := reviewdomain.ValidateReview(req, recordedAt)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if store.PlanDigest == nil {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("plan digest resolver is unavailable")
	}
	digest, err := store.PlanDigest(stateRoot, record)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	review.ReviewedPlanDigest = digest
	review, err = reviewdomain.ApplyReview(record.DevilsAdvocateReview, review)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record.DevilsAdvocateReview = &review
	return store.TouchWrite(stateRoot, record)
}

func RecordCompatibilityReview(store reviewport.CompatibilityStore, stateRoot, id string, req model.IssueOpsCompatibilityReviewRequest, recordedAt string) (model.IssueOpsRecord, error) {
	review, err := reviewdomain.ValidateCompatibilityReview(req, recordedAt)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := store.Ready(record); !ready.Ready {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot record compatibility review before plan/worktree readiness: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.CompatibilityReview = &review
	if store.PhaseRank(record.Phase) < store.PhaseRank(model.IssueOpsPhaseCompatibilityReview) {
		record.Phase = model.IssueOpsPhaseCompatibilityReview
	}
	return store.TouchWrite(stateRoot, record)
}
