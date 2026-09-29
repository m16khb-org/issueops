package stateio

import (
	statestore "issueops/internal/adapter/outbound/state"
	application "issueops/internal/application/selfaugment"
)

func PromoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (SelfAugmentPromoteResult, error) {
	return application.PromoteBaseline(fromKey, baselineKey, confirm, allowFailedSource, application.PromoteDeps{
		StateDir:      statestore.StateDir,
		ReadSnapshot:  ReadSelfAugmentStateSnapshot,
		WriteSnapshot: WriteSelfAugmentSnapshotRecord,
		ReadState:     statestore.StateRead,
	})
}
