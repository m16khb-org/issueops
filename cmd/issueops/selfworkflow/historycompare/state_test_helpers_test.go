package historycompare

import (
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	"time"
)

func writeSnapshotForTest(dir, key string, snapshot contract.SelfAugmentStateSnapshot) error {
	return (app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}).Write(dir, key, snapshot)
}
