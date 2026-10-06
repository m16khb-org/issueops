package selfworkflow

import (
	statestore "issueops/internal/adapter/outbound/state"
	application "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
)

func PromoteSelfAugmentBaseline(fromKey, baselineKey string, confirm, allowFailedSource bool) (contract.SelfAugmentPromoteResult, error) {
	return application.PromoteBaseline(fromKey, baselineKey, confirm, allowFailedSource, application.PromoteDeps{
		StateDir:      statestore.StateDir,
		ReadSnapshot:  ReadSelfAugmentStateSnapshot,
		WriteSnapshot: WriteSelfAugmentSnapshotRecord,
		ReadState:     statestore.NewService().Read,
	})
}
