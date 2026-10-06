package stateio

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	statecontract "issueops/internal/contract/state"
	"time"

	application "issueops/internal/application/selfaugment"
)

func snapshotStore() application.SnapshotStore {
	return application.SnapshotStore{ReadState: statestore.NewService().Read, NormalizeKey: statestore.NormalizeStateKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
		return statestore.WriteStateRecord(context.Background(), dir, key, record)
	}, Now: time.Now}
}

func ReadSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return snapshotStore().Read(key)
}
func WriteSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return snapshotStore().Write(dir, key, snapshot)
}
