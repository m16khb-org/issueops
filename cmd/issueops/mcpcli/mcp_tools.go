package mcpcli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/mcpcli/resources"
	apidocapp "issueops/internal/application/apidoc"
	auditapp "issueops/internal/application/audit"
	channelapp "issueops/internal/application/channel"
	commitapp "issueops/internal/application/commitsuggest"
	daemonapp "issueops/internal/application/daemon"
	gatesapp "issueops/internal/application/gates"
	lintapp "issueops/internal/application/lintdiagnose"
	loopapp "issueops/internal/application/looprun"
	policyapp "issueops/internal/application/policy"
	preflightapp "issueops/internal/application/preflight"
	bootstrapapp "issueops/internal/application/projectbootstrap"
	docsapp "issueops/internal/application/projectdocs"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	workerapp "issueops/internal/application/worker"
	authoritycontract "issueops/internal/contract/authority"
	executionissue "issueops/internal/contract/executionissue"
	inspectmodel "issueops/internal/contract/inspect"
	issueopscontract "issueops/internal/contract/issueops"
	mcpcontract "issueops/internal/contract/mcp"
	augmentcontract "issueops/internal/contract/selfaugment"
	webfetchmodel "issueops/internal/contract/webfetch"
	toolconformancedomain "issueops/internal/domain/toolconformance"
	"issueops/internal/port"
	authorityport "issueops/internal/port/authority"
	basesyncport "issueops/internal/port/issueopsbasesync"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

type MCPToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type MCPToolOutcome struct {
	Handled bool
	Direct  bool
	IsError bool
	Result  any
	Payload any
	Err     *jsonrpc.Error
}

// MCPDependencies는 server 생성 시 고정된다. 요청 간 package-global dependency
// cache를 두지 않아 서로 다른 MCP server의 handler가 섞이지 않는다.
type MCPDependencies struct {
	APIDoc        apidocapp.Service
	DefaultTarget string
	Inspect       func(repo, hostReceipts string) any
	Preflight     preflightapp.Service
	Skills        func(string, string) []inspectmodel.SkillInfo
	Compatibility func() any
	Commit        commitapp.Service
	Lint          lintapp.Service
	Fetch         func(context.Context, webfetchmodel.Request) (webfetchmodel.Result, error)
	Execution     ExecutionDeps

	Gates            gatesapp.Service
	Channel          channelapp.Service
	Policy           policyapp.Service
	Audit            auditapp.Service
	Daemon           daemonapp.Reader
	Worker           workerapp.Service
	Loop             loopapp.Service
	ProjectBootstrap bootstrapapp.Service
	ProjectDocs      docsapp.Service
	State            StateDependencies
	Resources        resources.Config
	SelfVerify       func(verifyapp.LoopRequest) (augmentcontract.SelfAugmentResult, error)
	Catalog          mcpcontract.Catalog
	SelfHistory      augmentapp.HistoryService
	SelfState        SelfStateDependencies
	SelfPlanning     SelfPlanningDependencies
	Prepare          issueopscontract.ExecutionPrepareHandler
	Orca             port.ExecutionOrcaProvisioner
	OrcaOwner        port.ExecutionOrcaOwnerInspector
	BaseSync         basesyncport.Inspector
	ReadIssue        executionissue.ExecutionIssueSnapshotReadFunc
	Claim            issueopscontract.ExecutionClaimHandler
	Release          issueopscontract.ExecutionReleaseHandler
	Status           port.ExecutionStatusHandler
	Replace          port.ExecutionReplaceHandler
	Reseed           issueopscontract.ExecutionReseedHandler
	Resume           issueopscontract.ExecutionResumeHandler
	Reconcile        port.ExecutionReconcileHandler
	Complete         issueopscontract.ExecutionCompleteHandler
	Publication      PublicationHandlers
	Provenance       provenanceport.Observer

	// Request authority: RequestScope -> Credentials.Read -> BindAuthority ->
	// ForRequest -> dispatch. ForRequest returns a per-request copy whose Caller
	// is the verified identity; server-scoped dependencies leave Caller nil.
	RequestScope  func(context.Context, string, map[string]any) (authoritycontract.Scope, error)
	Credentials   authorityport.CredentialFiles
	BindAuthority func(context.Context, authoritycontract.Use) (context.Context, issueopscontract.VerifiedActor, error)
	ForRequest    func(context.Context, authoritycontract.Scope, issueopscontract.VerifiedActor) (context.Context, MCPDependencies, error)
	BindTrace     func(context.Context, string) (context.Context, bool)
	Caller        *issueopscontract.VerifiedActor
}

func mcpToolPayload(payload any) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Payload: payload}
}

// mcpToolErrorPayload reports a tool-level FAILURE (not-found, validation,
// disk/lock, live-verify) as a normalized error tool result rather than a
// JSON-RPC protocol error: the payload is serialized as text content and the
// result is flagged isError, mirroring the CLI's {ok:false,error:...} body.
// Genuine JSON-RPC schema/param violations still use mcpToolFailure(-32602).
func mcpToolErrorPayload(payload any) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, IsError: true, Payload: payload}
}

func mcpToolDirect(result any) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Direct: true, Result: result}
}

func mcpToolFailure(err *jsonrpc.Error) MCPToolOutcome {
	return MCPToolOutcome{Handled: true, Err: err}
}

func newProtocolError(code int64, message string, data any) *jsonrpc.Error {
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	return &jsonrpc.Error{Code: code, Message: message, Data: raw}
}

func validateMCPToolArguments(catalog mcpcontract.Catalog, name string, arguments map[string]any) *jsonrpc.Error {
	for _, tool := range catalog.Tools {
		toolName, _ := tool["name"].(string)
		if toolName != name {
			continue
		}
		schema, ok := tool["inputSchema"].(map[string]any)
		if !ok {
			return newProtocolError(-32603, "Invalid tool schema", name)
		}
		diagnostics, err := toolconformancedomain.Validate(
			toolconformancedomain.ClosedProjection(schema),
			arguments,
		)
		if err != nil {
			return newProtocolError(-32603, "Invalid tool schema", fmt.Sprintf("%s: %v", name, err))
		}
		if len(diagnostics) == 0 {
			return nil
		}
		return newProtocolError(
			-32602,
			"Invalid params",
			toolconformancedomain.InvalidToolArgumentsResult(name, diagnostics),
		)
	}
	return newProtocolError(-32602, "Unknown tool", name)
}

func TextResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// ErrorTextResult is TextResult flagged as an MCP error result (isError:true),
// the tool-result form for tool-level failures that mirror the CLI body.
func ErrorTextResult(text string) map[string]any {
	result := TextResult(text)
	result["isError"] = true
	return result
}

func (deps MCPDependencies) resolveTarget(target string) string {
	if target != "" {
		return target
	}
	return deps.DefaultTarget
}
