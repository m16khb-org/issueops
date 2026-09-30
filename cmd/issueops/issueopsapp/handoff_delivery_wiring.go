package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	deliveryapp "issueops/internal/application/handoffdelivery"
	"issueops/internal/port"
)

func newHandoffDeliveryProvisioner(stateRoot string, next port.ExecutionOrcaProvisioner, now func() time.Time) port.ExecutionOrcaProvisioner {
	if next == nil {
		return nil
	}
	if now == nil {
		now = time.Now
	}
	return &deliveryapp.Provisioner{Audit: newHandoffDeliveryAudit(stateRoot), Next: next, Now: now}
}

func newHandoffDeliveryAudit(stateRoot string) auditadapter.HandoffDeliveryStore {
	return auditadapter.HandoffDeliveryStore{StateRoot: stateRoot, WithKeyLock: statestore.WithKeyLock}
}
