package selfworkflow

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	statepath "issueops/internal/domain/statepath"
	"time"

	application "issueops/internal/application/selfaugment"
)

func snapshotStore() application.SnapshotStore {
	return application.SnapshotStore{ReadState: statestore.NewService().Read, NormalizeKey: statepath.NormalizeKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
		return statestore.WriteStateRecord(context.Background(), dir, key, record)
	}, Now: time.Now}
}

func ReadSelfAugmentStateSnapshot(key string) (contract.SelfAugmentStateSnapshot, error) {
	return snapshotStore().Read(key)
}
func WriteSelfAugmentSnapshotRecord(dir, key string, snapshot contract.SelfAugmentStateSnapshot) error {
	return snapshotStore().Write(dir, key, snapshot)
}
