package mcpcli

import (
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/mcpcli/resources"
	"issueops/internal/adapter/docs"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/policy"
	stateapp "issueops/internal/application/state"
	"os"
)

func publicStateForTest() StateDependencies {
	stores := statestore.NewMaintenanceStores(statestore.StateDir(), os.Getenv("ISSUEOPS_WORKER_DIR"))
	maintenance := stateapp.NewMaintenanceService(stateapp.MaintenanceDependencies{AllRoots: stores.Roots, StoreExists: stores.Exists, MaintainStore: stores.Maintain})
	return StateDependencies{Write: statestore.StateWrite, Read: statestore.StateRead, List: statestore.StateList, Prune: statestore.StatePrune, Doctor: statestore.StateDoctor, Maintain: maintenance.Maintain}
}
func testHandlePolicyStateMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handlePolicyStateMCPToolCall(call, MCPDependencies{Policy: testPolicyService(), Audit: testAuditService(), State: publicStateForTest()})
}
func resourceConfigForTest() resources.Config {
	return resources.Config{IssueOpsRoot: IssueOpsRoot(), Version: Version, SkillName: skillName, ReadHarnessFile: ReadHarnessFile, StateList: publicStateForTest().List, RouteProjectDocs: testProjectDocsService().Route, DocsIndex: docs.DocsIndex, CommandPolicySummary: policy.CommandPolicySummary}
}
func testHandleResourceRead(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleResourceRead(params, resourceConfigForTest())
}
