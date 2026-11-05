// Package handoffdelivery is the application service that records handoff delivery
// observations (manual confirmations and owner claims) in the delivery audit.
package handoffdelivery

import (
	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// Audit is the append-only store of handoff delivery observations.
type Audit interface {
	Append(model.HandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)
	Read() ([]model.HandoffDeliveryObservation, error)
}

// Service validates handoff delivery observations against lifecycle records and
// appends them to Audit.
type Service struct {
	ReadRecord func(string) (model.Record, error)
	Audit      Audit
}

// ObserveManual reads the lifecycle record named by observation, validates the
// manual delivery observation against it, and appends it to the audit.
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

// ObserveClaim records an owner-claim delivery observation derived from result.
// It does nothing when the result is not claim-observable or no observation is
// needed given the existing audit entries.
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
