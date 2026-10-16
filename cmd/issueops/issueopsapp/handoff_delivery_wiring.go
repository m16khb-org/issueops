package issueopsapp

import (
	statestore "issueops/internal/adapter/outbound/state"

	auditadapter "issueops/internal/adapter/audit"
)

func newHandoffDeliveryAudit(stateRoot string) auditadapter.HandoffDeliveryStore {
	return auditadapter.HandoffDeliveryStore{StateRoot: stateRoot, WithKeyLock: statestore.WithKeyLock}
}
