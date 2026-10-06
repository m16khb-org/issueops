package issueopsreview

import (
	"errors"
	"fmt"
	intentapp "issueops/internal/application/issueopsintent"
	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	domain "issueops/internal/domain/issueopsreview"
	intentport "issueops/internal/port/issueopsintent"
	port "issueops/internal/port/issueopsreview"
)

// PlanningRecorder keeps authority checks and planning mutations under one lock.
type PlanningRecorder struct {
	ActiveChildren         func(string, model.IssueOpsRecord) ([]string, error)
	Store                  port.ReviewMutationStore
	PlanReadiness          func(model.IssueOpsRecord) model.IssueOpsReadiness
	CompatibilityReadiness func(model.IssueOpsRecord) model.IssueOpsReadiness
	PhaseRank              func(model.IssueOpsPhase) int
	PlanDigest             func(string, model.IssueOpsRecord) (string, error)
}

func (s PlanningRecorder) record(root, id string, apply func() (model.IssueOpsRecord, error)) (model.IssueOpsRecord, error) {
	var result model.IssueOpsRecord
	err := s.Store.WithLock(root, id, func() error {
		current, err := s.Store.Read(root, id)
		if err != nil {
			return err
		}
		if err = s.Store.ValidateMutation(current); err != nil {
			return err
		}
		result, err = apply()
		return err
	})
	return result, err
}
func (s PlanningRecorder) touchWrite(root string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	record.UpdatedAt = s.Store.Now()
	return s.Store.Write(root, record)
}
func (s PlanningRecorder) Intent(root, id string, req model.IssueOpsIntentRecordRequest) (model.IssueOpsRecord, error) {
	return s.record(root, id, func() (model.IssueOpsRecord, error) {
		return intentapp.RecordIntent(intentport.Store{Read: s.Store.Read, TouchWrite: s.touchWrite, Now: s.Store.Now}, root, id, req)
	})
}
func (s PlanningRecorder) PlanPrep(root, id string, req model.IssueOpsPlanPrepRequest) (model.IssueOpsRecord, error) {
	return s.record(root, id, func() (model.IssueOpsRecord, error) {
		return intentapp.RecordPlanPrep(intentport.Store{Read: s.Store.Read, TouchWrite: s.touchWrite, Now: s.Store.Now}, root, id, req)
	})
}
func (s PlanningRecorder) Design(root, id string, req reviewcontract.DesignReviewRequest) (model.IssueOpsRecord, error) {
	return s.record(root, id, func() (model.IssueOpsRecord, error) {
		record, err := RecordDesignReview(port.DesignReviewStore{Read: s.Store.Read, TouchWrite: s.touchWrite, Now: s.Store.Now, PlanReadiness: s.PlanReadiness}, root, id, req)
		if errors.Is(err, domain.ErrMissingDesignReviewEvidence) {
			return model.IssueOpsRecord{OK: false}, errors.New(`approved design review requires design_review_evidence: this is not a separate flag or decision record; add --verification "design review checked alternatives and risks" or a Korean equivalent such as "설계 검토 완료: 대안과 위험 확인"`)
		}
		return record, err
	})
}
func (s PlanningRecorder) Compatibility(root, id string, req reviewcontract.CompatibilityReviewRequest) (model.IssueOpsRecord, error) {
	return s.record(root, id, func() (model.IssueOpsRecord, error) {
		return RecordCompatibilityReview(port.CompatibilityStore{Read: s.Store.Read, TouchWrite: s.touchWrite, Ready: s.CompatibilityReadiness, PhaseRank: s.PhaseRank}, root, id, req, s.Store.Now())
	})
}
func (s PlanningRecorder) DevilsAdvocate(root, id string, req reviewcontract.DevilsAdvocateReviewRequest) (model.IssueOpsRecord, error) {
	return s.record(root, id, func() (model.IssueOpsRecord, error) {
		record, err := RecordDevilsAdvocate(port.DevilsAdvocateStore{Read: s.Store.Read, TouchWrite: s.touchWrite, PlanDigest: s.PlanDigest}, root, id, req, s.Store.Now())
		if capErr, ok := errors.AsType[*domain.ReviseRoundCapError](err); ok {
			return model.IssueOpsRecord{OK: false}, fmt.Errorf("revise round cap reached: cycle %s already recorded %d unwaived revise verdicts on this plan phase, so the plan is not converging; "+
				"record a stop verdict, reflect it with issueops remote reflect-devils-advocate --id %s --confirm, then issueops regress --id %s --reason <TEXT>; "+
				"or record this round with --waive --waiver-rationale <TEXT>", id, capErr.Count, id, id)
		}
		return record, err
	})
}

func (s PlanningRecorder) Regress(root, id, reason string) (model.IssueOpsRecord, error) {
	reason, err := domain.NormalizeRegressionReason(reason)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return s.record(root, id, func() (model.IssueOpsRecord, error) {
		return Regress(port.RegressStore{Read: s.Store.Read, ActiveChildren: s.ActiveChildren, TouchWrite: s.touchWrite, Now: s.Store.Now}, root, id, reason)
	})
}
