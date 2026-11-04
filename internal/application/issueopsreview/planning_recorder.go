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
	ActiveChildren         func(string, model.Record) ([]string, error)
	Store                  port.ReviewMutationStore
	PlanReadiness          func(model.Record) model.Readiness
	CompatibilityReadiness func(model.Record) model.Readiness
	PhaseRank              func(model.Phase) int
	PlanDigest             func(string, model.Record) (string, error)
	ArchitectureDocument   func(string, model.Record) (string, error)
}

func (s PlanningRecorder) record(root, id string, apply func() (model.Record, error)) (model.Record, error) {
	var result model.Record
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
func (s PlanningRecorder) touchWrite(root string, record model.Record) (model.Record, error) {
	record.UpdatedAt = s.Store.Now()
	return s.Store.Write(root, record)
}
func (s PlanningRecorder) Intent(root, id string, req model.IntentRecordRequest) (model.Record, error) {
	return s.record(root, id, func() (model.Record, error) {
		return intentapp.RecordIntent(intentport.Store{Read: s.Store.Read, TouchWrite: s.touchWrite, Now: s.Store.Now}, root, id, req)
	})
}
func (s PlanningRecorder) PlanPrep(root, id string, req model.PlanPrepRequest) (model.Record, error) {
	return s.record(root, id, func() (model.Record, error) {
		return intentapp.RecordPlanPrep(intentport.Store{Read: s.Store.Read, TouchWrite: s.touchWrite, Now: s.Store.Now}, root, id, req)
	})
}
func (s PlanningRecorder) Architecture(root, id string, req reviewcontract.ArchitectureReviewRequest) (model.Record, error) {
	return s.record(root, id, func() (model.Record, error) {
		record, err := RecordArchitectureReview(port.ArchitectureReviewStore{Read: s.Store.Read, TouchWrite: s.touchWrite, Now: s.Store.Now, PlanReadiness: s.PlanReadiness, Document: s.ArchitectureDocument}, root, id, req)
		if errors.Is(err, domain.ErrMissingArchitectureReviewEvidence) {
			return model.Record{OK: false}, errors.New(`approved architecture review requires architecture_review_evidence: this is not a separate flag or decision record; add --verification "architecture review checked alternatives and risks" or a Korean equivalent such as "아키텍처 검토 완료: 대안과 위험 확인"`)
		}
		return record, err
	})
}
func (s PlanningRecorder) Compatibility(root, id string, req reviewcontract.CompatibilityReviewRequest) (model.Record, error) {
	return s.record(root, id, func() (model.Record, error) {
		return RecordCompatibilityReview(port.CompatibilityStore{Read: s.Store.Read, TouchWrite: s.touchWrite, Ready: s.CompatibilityReadiness, PhaseRank: s.PhaseRank}, root, id, req, s.Store.Now())
	})
}
func (s PlanningRecorder) DevilsAdvocate(root, id string, req reviewcontract.DevilsAdvocateReviewRequest) (model.Record, error) {
	return s.record(root, id, func() (model.Record, error) {
		record, err := RecordDevilsAdvocate(port.DevilsAdvocateStore{Read: s.Store.Read, TouchWrite: s.touchWrite, PlanDigest: s.PlanDigest}, root, id, req, s.Store.Now())
		if capErr, ok := errors.AsType[*domain.ReviseRoundCapError](err); ok {
			return model.Record{OK: false}, fmt.Errorf("revise round cap reached: cycle %s already recorded %d unwaived revise verdicts on this plan phase, so the plan is not converging; "+
				"record a stop verdict, reflect it with issueops remote reflect-devils-advocate --id %s --confirm, then issueops regress --id %s --reason <TEXT>; "+
				"or record this round with --waive --waiver-rationale <TEXT>", id, capErr.Count, id, id)
		}
		return record, err
	})
}

func (s PlanningRecorder) Regress(root, id, reason string) (model.Record, error) {
	reason, err := domain.NormalizeRegressionReason(reason)
	if err != nil {
		return model.Record{OK: false}, err
	}
	return s.record(root, id, func() (model.Record, error) {
		return Regress(port.RegressStore{Read: s.Store.Read, ActiveChildren: s.ActiveChildren, TouchWrite: s.touchWrite, Now: s.Store.Now}, root, id, reason)
	})
}
