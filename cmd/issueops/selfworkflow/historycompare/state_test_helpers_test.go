package historycompare

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	statepath "issueops/internal/domain/statepath"
	"time"
)

func writeSnapshotForTest(dir, key string, snapshot contract.SelfAugmentStateSnapshot) error {
	return (app.SnapshotStore{NormalizeKey: statepath.NormalizeKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
		return statestore.WriteStateRecord(context.Background(), dir, key, record)
	}, Now: time.Now}).Write(dir, key, snapshot)
}
