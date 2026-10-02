package mcpcli

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	mcpcontract "issueops/internal/contract/mcp"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"testing"
	"time"
)

func testMCPCatalog() mcpcontract.Catalog { return mcpcatalog.Build() }

func testCallSDKTool(t *testing.T, params json.RawMessage) (any, *jsonrpc.Error) {
	t.Helper()
	deps := testTransportServices()
	deps.Gates = testGatesService()
	deps.Channel = testChannelService()
	deps.Policy = testPolicyService()
	deps.Audit = testAuditService()
	deps.Loop = testLoopService()
	deps.ProjectDocs = testProjectDocsService()
	deps.ProjectBootstrap = testBootstrapService()
	deps.State = publicStateForTest()
	deps.SelfHistory = historyServiceForTest()
	deps.SelfState = selfStateForTest()
	deps.SelfPlanning = planningForTest(IssueOpsRoot(), statestore.StateDir(), Version)
	return callSDKTool(t, params, deps)
}

func callSDKTool(t *testing.T, params json.RawMessage, deps MCPDependencies) (any, *jsonrpc.Error) {
	t.Helper()
	var call mcp.CallToolParams
	if err := json.Unmarshal(params, &call); err != nil {
		t.Fatal(err)
	}
	session := startMCPTransportTestSession(t, "stdio", deps)
	result, err := session.CallTool(t.Context(), &call)
	if err != nil {
		var protocolErr *jsonrpc.Error
		if !errors.As(err, &protocolErr) {
			t.Fatalf("SDK call returned non-protocol error: %v", err)
		}
		return nil, protocolErr
	}
	content := make([]map[string]any, 0, len(result.Content))
	for _, item := range result.Content {
		text, ok := item.(*mcp.TextContent)
		if !ok {
			t.Fatalf("unexpected SDK content: %T", item)
		}
		content = append(content, map[string]any{"type": "text", "text": text.Text})
	}
	envelope := map[string]any{"content": content}
	if result.IsError {
		envelope["isError"] = true
	}
	return envelope, nil
}

func historyServiceForTest() augmentapp.HistoryService {
	return augmentapp.HistoryService{StateDir: statestore.StateDir, List: statestore.StateList, Read: statestore.StateRead, Delete: statestore.StateDelete}
}
func testHandleSelfLoopMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handleSelfLoopMCPToolCall(context.Background(), call, MCPDependencies{SelfHistory: historyServiceForTest(), SelfState: selfStateForTest(), SelfPlanning: planningForTest(IssueOpsRoot(), statestore.StateDir(), Version)})
}

func selfStateForTest() SelfStateDependencies {
	snapshots := augmentapp.SnapshotStore{ReadState: statestore.StateRead, NormalizeKey: statestore.NormalizeStateKey, WriteRecord: func(dir, key string, record statecontract.RecordEnvelope) (string, error) {
		return statestore.WriteStateRecord(context.Background(), dir, key, record)
	}, Now: time.Now}
	return SelfStateDependencies{
		SavePlan: func(_ context.Context, result *contract.SelfAugmentPlanResult, key string) error {
			return augmentapp.SavePlan(result, key, augmentapp.SavePlanDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentPlanStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: func(key, content string) (statecontract.StateResult, error) {
				return statestore.StateWrite(context.Background(), key, content)
			}, StateDir: statestore.StateDir})
		},
		SaveSummary: func(_ context.Context, result *contract.SelfAugmentResult, key string) error {
			return verifyapp.SaveSummary(result, key, verifyapp.SaveSummaryDeps{Now: time.Now, Encode: func(snapshot contract.SelfAugmentStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: func(key, content string) (statecontract.StateResult, error) {
				return statestore.StateWrite(context.Background(), key, content)
			}, StateDir: statestore.StateDir})
		},
		Promote: func(_ context.Context, from, to string, confirm, allowFailed bool) (contract.SelfAugmentPromoteResult, error) {
			return augmentapp.PromoteBaseline(from, to, confirm, allowFailed, augmentapp.PromoteDeps{StateDir: statestore.StateDir, ReadSnapshot: snapshots.Read, WriteSnapshot: snapshots.Write, ReadState: statestore.StateRead})
		},
	}
}
