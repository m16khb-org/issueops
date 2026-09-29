package issueopsapp

import (
	"encoding/json"
	"issueops/cmd/issueops/mcpcli"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	stateapp "issueops/internal/application/state"
	contract "issueops/internal/contract/selfaugment"
	"issueops/internal/domain/statepath"
	stateport "issueops/internal/port/state"
	"time"
)

func newSelfWorkflowStateService(dir string) *stateapp.Service {
	return stateapp.NewService(stateapp.Dependencies{
		StateDir: func() string { return dir }, StatePath: statepath.Path,
		OpenStore:       func(dir string) (stateport.Store, error) { return sqlstore.Open(dir) },
		ExistingRecords: statestore.ExistingRecords{},
	})
}

func newSelfWorkflowState(dir string) mcpcli.SelfStateDependencies {
	state := newSelfWorkflowStateService(dir)
	stateDir := func() string { return dir }
	snapshots := augmentapp.SnapshotStore{ReadState: state.Read, NormalizeKey: statepath.NormalizeKey, WriteRecord: state.WriteRecord, Now: time.Now}
	return mcpcli.SelfStateDependencies{
		SavePlan: func(result *contract.SelfAugmentPlanResult, key string) error {
			return augmentapp.SavePlan(result, key, augmentapp.SavePlanDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentPlanStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: state.Write, StateDir: stateDir})
		},
		SaveSummary: func(result *contract.SelfAugmentResult, key string) error {
			return verifyapp.SaveSummary(result, key, verifyapp.SaveSummaryDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: state.Write, StateDir: stateDir})
		},
		Promote: func(from, to string, confirm, allowFailed bool) (contract.SelfAugmentPromoteResult, error) {
			return augmentapp.PromoteBaseline(from, to, confirm, allowFailed, augmentapp.PromoteDeps{StateDir: stateDir, ReadSnapshot: snapshots.Read, WriteSnapshot: snapshots.Write, ReadState: state.Read})
		},
	}
}
