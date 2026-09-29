package issueopsapp

import (
	auditadapter "issueops/internal/adapter/audit"
	issueopsadapter "issueops/internal/adapter/issueops"
	deliveryapp "issueops/internal/application/handoffdelivery"
	model "issueops/internal/contract/issueops"
)

func newHandoffDeliveryService(stateRoot string) deliveryapp.Service {
	return deliveryapp.Service{
		ReadRecord: func(id string) (model.IssueOpsRecord, error) { return issueopsadapter.ReadIssueOps(stateRoot, id) },
		Audit:      auditadapter.HandoffDeliveryStore{StateRoot: stateRoot},
	}
}
