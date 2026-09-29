package handoffdelivery

import (
	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type Audit interface {
	Append(model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)
	Read() ([]model.IssueOpsHandoffDeliveryObservation, error)
	ReadFor(string, string) ([]model.IssueOpsHandoffDeliveryObservation, error)
}

type Service struct {
	ReadRecord func(string) (model.IssueOpsRecord, error)
	Audit      Audit
}

func (s Service) ObserveManual(observation model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	record, err := s.ReadRecord(observation.LifecycleID)
	if err != nil {
		return auditcontract.HandoffDeliveryAuditRecord{}, err
	}
	return s.ObserveManualSnapshot(record, observation)
}

// ObserveManualSnapshot uses the caller's fenced record without another read.
func (s Service) ObserveManualSnapshot(record model.IssueOpsRecord, observation model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	if err := domain.ValidateManualHandoffDeliveryObservation(record, observation); err != nil {
		return auditcontract.HandoffDeliveryAuditRecord{}, err
	}
	return s.Audit.Append(observation)
}

func (s Service) ObserveClaim(result model.ExecutionResult) error {
	query := domain.HandoffDeliveryClaimQueryFor(result)
	if !query.Observe {
		return nil
	}
	var observations []model.IssueOpsHandoffDeliveryObservation
	var err error
	if query.Manual {
		observations, err = s.Audit.Read()
	} else {
		observations, err = s.Audit.ReadFor(query.LifecycleID, query.LineageID)
	}
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
