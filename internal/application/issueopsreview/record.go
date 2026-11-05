// Package issueopsreview is the application layer that records review evidence on
// an issueops record: architecture, domain, compatibility, devil's advocate,
// project docs, schema and AI-slop-clean reviews, feedback, and plan regressions.
// It validates requests through the review domain and persists through ports.
package issueopsreview

import (
	"fmt"
	reviewcontract "issueops/internal/contract/issueopsreview"
	"strings"

	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordDevilsAdvocate validates req, binds the review to the digest of the
// current plan and stores it on record id. It fails when the plan digest resolver
// is missing or the review domain rejects the round.
func RecordDevilsAdvocate(store reviewport.DevilsAdvocateStore, stateRoot, id string, req reviewcontract.DevilsAdvocateReviewRequest, recordedAt string) (model.Record, error) {
	review, err := reviewdomain.ValidateReview(req, recordedAt)
	if err != nil {
		return model.Record{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if store.PlanDigest == nil {
		return model.Record{OK: false}, fmt.Errorf("plan digest resolver is unavailable")
	}
	digest, err := store.PlanDigest(stateRoot, record)
	if err != nil {
		return model.Record{OK: false}, err
	}
	review.ReviewedPlanDigest = digest
	review, err = reviewdomain.ApplyReview(record.DevilsAdvocateReview, review)
	if err != nil {
		return model.Record{OK: false}, err
	}
	record.DevilsAdvocateReview = &review
	return store.TouchWrite(stateRoot, record)
}

// RecordCompatibilityReview validates req and stores it on record id, advancing
// the phase to compatibility review when it is earlier. It fails before the
// plan and worktree are ready.
func RecordCompatibilityReview(store reviewport.CompatibilityStore, stateRoot, id string, req reviewcontract.CompatibilityReviewRequest, recordedAt string) (model.Record, error) {
	review, err := reviewdomain.ValidateCompatibilityReview(req, recordedAt)
	if err != nil {
		return model.Record{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if ready := store.Ready(record); !ready.Ready {
		return model.Record{OK: false}, fmt.Errorf("cannot record compatibility review before plan/worktree readiness: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.CompatibilityReview = &review
	if store.PhaseRank(record.Phase) < store.PhaseRank(model.PhaseCompatibilityReview) {
		record.Phase = model.PhaseCompatibilityReview
	}
	return store.TouchWrite(stateRoot, record)
}
