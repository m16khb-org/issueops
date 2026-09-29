package issueopsapp

import (
	"time"

	statestore "issueops/internal/adapter/outbound/state"
	probe "issueops/internal/adapter/verification/probe"
	stateroundtrip "issueops/internal/adapter/verification/probe/stateroundtrip"
	stepbudget "issueops/internal/adapter/verification/probe/stepbudget"
	selfaugmentapp "issueops/internal/application/selfaugment"
)

// configureStateStores는 issueops state 접근을 설치한다.
//
// state를 어디에 저장하고 어떻게 잠그는지는 하나의 구현이고, 그 선택은
// composition root의 결정이다. CLI/MCP transport와 self-workflow는 key와 결과
// 형식만 안다.
func configureStateStores() {
	snapshotStore := selfaugmentapp.SnapshotStore{NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
	probe.WriteSnapshot = snapshotStore.Write
	stateroundtrip.WriteSnapshot = snapshotStore.Write
	stepbudget.WriteSnapshot = snapshotStore.Write

	stateroundtrip.StateRead = statestore.StateRead
	stateroundtrip.WriteStateRecord = statestore.WriteStateRecord
}
