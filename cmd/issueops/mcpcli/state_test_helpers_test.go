package mcpcli

import (
	"context"
	"encoding/json"
	"issueops/cmd/issueops/mcpcli/resources"
	"issueops/internal/adapter/docs"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/policy"
	docsapp "issueops/internal/application/docs"
	stateapp "issueops/internal/application/state"
	statecontract "issueops/internal/contract/state"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func publicStateForTest() StateDependencies {
	stores := statestore.NewMaintenanceStores(statestore.StateDir(), os.Getenv("ISSUEOPS_WORKER_DIR"))
	maintenance := stateapp.NewMaintenanceService(stateapp.MaintenanceDependencies{AllRoots: stores.Roots, StoreExists: stores.Exists, MaintainStore: stores.Maintain})
	service := statestore.NewService()
	doctor := func() (statecontract.StateDoctorResult, error) { return statestore.Doctor(statestore.StateDir()) }
	return StateDependencies{Write: service.Write, Read: service.Read, List: service.List, Prune: service.Prune, Doctor: doctor, Maintain: maintenance.Maintain}
}
func testHandlePolicyStateMCPToolCall(call MCPToolCall) MCPToolOutcome {
	return handlePolicyStateMCPToolCall(context.Background(), call, MCPDependencies{Policy: testPolicyService(), Audit: testAuditService(), State: publicStateForTest()})
}
func resourceConfigForTest() resources.Config {
	return resources.Config{IssueOpsRoot: IssueOpsRoot(), Version: Version, SkillName: skillName, ReadHarnessFile: ReadHarnessFile, StateList: publicStateForTest().List, RouteProjectDocs: testProjectDocsService().Route, DocsIndex: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index, CommandPolicySummary: policy.CommandPolicySummary}
}
func testHandleResourceRead(params json.RawMessage) (any, *jsonrpc.Error) {
	return HandleResourceRead(params, resourceConfigForTest())
}
