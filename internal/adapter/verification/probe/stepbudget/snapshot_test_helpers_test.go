package stepbudget

import (
	"time"

	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
)

func init() {
	store := app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
	WriteSnapshot = store.Write
}
