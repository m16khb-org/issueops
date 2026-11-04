package audit

import "context"

// HandoffDeliveryStore reads and appends the handoff delivery audit log under StateRoot. WithKeyLock
// serializes access across processes by (state root, key); it is called with the key "handoff-delivery-audit".
type HandoffDeliveryStore struct {
	StateRoot   string
	WithKeyLock func(context.Context, string, string, func(context.Context) error) error
	openHooks   handoffDeliveryOpenHooks
}
