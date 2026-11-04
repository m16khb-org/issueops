package handoffdelivery

import (
	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type Audit interface {
	Append(model.HandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)
	Read() ([]model.HandoffDeliveryObservation, error)
}

type Service struct {
	ReadRecord func(string) (model.Record, error)
	Audit      Audit
}

func (s Service) ObserveManual(observation model.HandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	record, err := s.ReadRecord(observation.LifecycleID)
	if err != nil {
		return auditcontract.HandoffDeliveryAuditRecord{}, err
	}
	return s.ObserveManualSnapshot(record, observation)
}

// ObserveManualSnapshot uses the caller's fenced record without another read.
func (s Service) ObserveManualSnapshot(record model.Record, observation model.HandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	if err := domain.ValidateManualHandoffDeliveryObservation(record, observation); err != nil {
		return auditcontract.HandoffDeliveryAuditRecord{}, err
	}
	return s.Audit.Append(observation)
}

func (s Service) ObserveClaim(result model.ExecutionResult) error {
	if !domain.HandoffDeliveryClaimObservable(result) {
		return nil
	}
	observations, err := s.Audit.Read()
	if err != nil {
		return err
	}
	observation, err := domain.OwnerClaimHandoffDeliveryObservation(result, observations)
	if err != nil || observation == nil {
		return err
	}
	_, err = s.Audit.Append(*observation)
	return err
}
