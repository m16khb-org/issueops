package audit

import "context"

type HandoffDeliveryStore struct {
	StateRoot   string
	WithKeyLock func(context.Context, string, string, func(context.Context) error) error
	openHooks   handoffDeliveryOpenHooks
}
