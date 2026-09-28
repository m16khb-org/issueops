package port

import (
	"context"

	"issueops/internal/contract/issueops"
)

type BaseSyncPushEffects interface {
	Push(context.Context, string, string) (int, string)
	AppendEvent(context.Context, string, issueops.ExecutionSyncBaseEvent) error
}
