package audit

import (
	"context"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// Filesystem tests deliberately execute callbacks directly; root integration
// tests exercise the real key lock. No package-global dependency is installed.
func auditStoreForTest(root string) HandoffDeliveryStore {
	return HandoffDeliveryStore{StateRoot: root, WithKeyLock: func(ctx context.Context, _ string, _ string, fn func(context.Context) error) error { return fn(ctx) }}
}

func foldHandoffDeliveryForTest(store HandoffDeliveryStore, lifecycleID, lineageID string) (map[string]model.IssueOpsHandoffDeliveryObservation, []model.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := store.ReadFor(lifecycleID, lineageID)
	folded, decisions := domain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}
