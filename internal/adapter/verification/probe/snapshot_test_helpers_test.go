package probe

import (
	"time"

	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification/probe/stateroundtrip"
	"issueops/internal/adapter/verification/probe/stepbudget"
	app "issueops/internal/application/selfaugment"
)

func init() {
	store := app.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
	WriteSnapshot = store.Write
	stateroundtrip.WriteSnapshot = store.Write
	stepbudget.WriteSnapshot = store.Write
}
