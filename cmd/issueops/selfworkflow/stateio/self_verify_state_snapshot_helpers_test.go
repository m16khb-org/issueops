package stateio

import (
	statestore "issueops/internal/adapter/outbound/state"
	"time"

	application "issueops/internal/application/selfaugment"
	domain "issueops/internal/domain/selfaugment"
)

func snapshotStore() application.SnapshotStore {
	return application.SnapshotStore{ReadState: statestore.StateRead, NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
}

func ReadSelfAugmentStateSnapshot(key string) (SelfAugmentStateSnapshot, error) {
	return snapshotStore().Read(key)
}
func WriteSelfAugmentSnapshotRecord(dir, key string, snapshot SelfAugmentStateSnapshot) error {
	return snapshotStore().Write(dir, key, snapshot)
}
func IsSelfVerificationSummaryKind(kind string) bool {
	return domain.IsSelfVerificationSummaryKind(kind)
}
func NormalizeSelfAugmentSnapshotFailureCause(snapshot *SelfAugmentStateSnapshot) {
	application.NormalizeSnapshotFailureCause(snapshot)
}
