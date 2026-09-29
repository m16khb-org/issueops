package mcpcli

import (
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/contractcli"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	mcpcontract "issueops/internal/contract/mcp"
	contract "issueops/internal/contract/selfaugment"
	"time"
)

func testMCPCatalog() mcpcontract.Catalog { return mcpcatalog.Build() }

func testHandleToolCall(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleToolCallWithDependencies(params, MCPDependencies{Catalog: testMCPCatalog(), SelfHistory: historyServiceForTest(), SelfState: selfStateForTest(), SelfPlanning: planningForTest(IssueOpsRoot(), statestore.StateDir(), Version)})
}

func init() {
	CompatibilityContract = func() any {
		return contractcli.BuildCompatibilityContract(clicatalog.Commands(), testMCPCatalog().Tools)
	}
}

func historyServiceForTest() augmentapp.HistoryService {
	return augmentapp.HistoryService{StateDir: statestore.StateDir, List: statestore.StateList, Read: statestore.StateRead, Delete: statestore.StateDelete}
}
func testHandleSelfLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleSelfLoopMCPToolCall(call, MCPDependencies{SelfHistory: historyServiceForTest(), SelfState: selfStateForTest(), SelfPlanning: planningForTest(IssueOpsRoot(), statestore.StateDir(), Version)})
}

func selfStateForTest() SelfStateDependencies {
	snapshots := augmentapp.SnapshotStore{ReadState: statestore.StateRead, NormalizeKey: statestore.NormalizeStateKey, WriteRecord: statestore.WriteStateRecord, Now: time.Now}
	return SelfStateDependencies{
		SavePlan: func(result *contract.SelfAugmentPlanResult, key string) error {
			return augmentapp.SavePlan(result, key, augmentapp.SavePlanDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentPlanStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: statestore.StateWrite, StateDir: statestore.StateDir})
		},
		SaveSummary: func(result *contract.SelfAugmentResult, key string) error {
			return verifyapp.SaveSummary(result, key, verifyapp.SaveSummaryDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: statestore.StateWrite, StateDir: statestore.StateDir})
		},
		Promote: func(from, to string, confirm, allowFailed bool) (contract.SelfAugmentPromoteResult, error) {
			return augmentapp.PromoteBaseline(from, to, confirm, allowFailed, augmentapp.PromoteDeps{StateDir: statestore.StateDir, ReadSnapshot: snapshots.Read, WriteSnapshot: snapshots.Write, ReadState: statestore.StateRead})
		},
	}
}
