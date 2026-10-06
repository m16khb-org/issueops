package stateio

import (
	statestore "issueops/internal/adapter/outbound/state"
	application "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func PromoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (augmentcontract.SelfAugmentPromoteResult, error) {
	return application.PromoteBaseline(fromKey, baselineKey, confirm, allowFailedSource, application.PromoteDeps{
		StateDir:      statestore.StateDir,
		ReadSnapshot:  ReadSelfAugmentStateSnapshot,
		WriteSnapshot: WriteSelfAugmentSnapshotRecord,
		ReadState:     statestore.NewService().Read,
	})
}
