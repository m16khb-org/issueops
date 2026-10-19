package intentarchitecture

import (
	"errors"
	reviewcontract "issueops/internal/contract/issueopsreview"
	"time"

	intentapp "issueops/internal/application/issueopsintent"
	reviewapp "issueops/internal/application/issueopsreview"
	model "issueops/internal/contract/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	intentport "issueops/internal/port/issueopsintent"
	reviewport "issueops/internal/port/issueopsreview"
)

type Store struct {
	Read          func(stateRoot, id string) (model.IssueOpsRecord, error)
	TouchWrite    func(stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error)
	PlanReadiness func(record model.IssueOpsRecord) model.IssueOpsReadiness
	Document      func(stateRoot string, record model.IssueOpsRecord) (string, error)
}

const (
	architectureReviewEvidenceGuidance = `approved architecture review requires architecture_review_evidence: this is not a separate flag or decision record; add --verification "architecture review checked alternatives and risks" or a Korean equivalent such as "아키텍처 검토 완료: 대안과 위험 확인"`
)

func RecordIntent(store Store, stateRoot, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	return intentapp.RecordIntent(intentport.Store{
		Read:       store.Read,
		TouchWrite: store.TouchWrite,
		Now:        func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
	}, stateRoot, id, req)
}

func RecordArchitectureReview(store Store, stateRoot, id string, req reviewcontract.ArchitectureReviewRequest) (model.IssueOpsRecord, error) {
	record, err := reviewapp.RecordArchitectureReview(reviewport.ArchitectureReviewStore{
		Read:          store.Read,
		PlanReadiness: store.PlanReadiness,
		TouchWrite:    store.TouchWrite,
		Now:           func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
		Document:      store.Document,
	}, stateRoot, id, req)
	if errors.Is(err, reviewdomain.ErrMissingArchitectureReviewEvidence) {
		return model.IssueOpsRecord{OK: false}, errors.New(architectureReviewEvidenceGuidance)
	}
	return record, err
}
