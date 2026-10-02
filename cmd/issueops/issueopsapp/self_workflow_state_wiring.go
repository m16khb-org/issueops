package issueopsapp

import (
	"context"
	"encoding/json"
	"issueops/cmd/issueops/mcpcli"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	stateapp "issueops/internal/application/state"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"issueops/internal/domain/statepath"
	"time"
)

func newSelfWorkflowState(dir string) mcpcli.SelfStateDependencies {
	state := newStateService(dir)
	stateDir := func() string { return dir }
	return mcpcli.SelfStateDependencies{
		SavePlan: func(ctx context.Context, result *contract.SelfAugmentPlanResult, key string) error {
			return augmentapp.SavePlan(result, key, augmentapp.SavePlanDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentPlanStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: requestStateWrite(ctx, state), StateDir: stateDir})
		},
		SaveSummary: func(ctx context.Context, result *contract.SelfAugmentResult, key string) error {
			return verifyapp.SaveSummary(result, key, verifyapp.SaveSummaryDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: requestStateWrite(ctx, state), StateDir: stateDir})
		},
		Promote: func(ctx context.Context, from, to string, confirm, allowFailed bool) (contract.SelfAugmentPromoteResult, error) {
			snapshots := augmentapp.SnapshotStore{ReadState: state.Read, NormalizeKey: statepath.NormalizeKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
				return state.WriteRecord(ctx, dir, key, record)
			}, Now: time.Now}
			return augmentapp.PromoteBaseline(from, to, confirm, allowFailed, augmentapp.PromoteDeps{StateDir: stateDir, ReadSnapshot: snapshots.Read, WriteSnapshot: snapshots.Write, ReadState: state.Read})
		},
	}
}

func requestStateWrite(ctx context.Context, state *stateapp.Service) func(string, string) (statecontract.StateResult, error) {
	return func(key, content string) (statecontract.StateResult, error) { return state.Write(ctx, key, content) }
}
