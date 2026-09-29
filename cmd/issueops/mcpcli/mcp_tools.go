package mcpcli

import (
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/cmd/issueops/mcpcli/resources"
	docsapp "issueops/internal/application/projectdocs"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	executionissue "issueops/internal/contract/executionissue"
	issueopscontract "issueops/internal/contract/issueops"
	mcpcontract "issueops/internal/contract/mcp"
	augmentcontract "issueops/internal/contract/selfaugment"
	toolconformancedomain "issueops/internal/domain/toolconformance"
	"issueops/internal/port"
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
	ProjectDocs  docsapp.Service
	State        StateDependencies
	Resources    resources.Config
	SelfVerify   func(verifyapp.LoopRequest) (augmentcontract.SelfAugmentResult, error)
	Catalog      mcpcontract.Catalog
	SelfHistory  augmentapp.HistoryService
	SelfState    SelfStateDependencies
	SelfPlanning SelfPlanningDependencies
	Prepare      issueopscontract.ExecutionPrepareHandler
	Orca         port.ExecutionOrcaProvisioner
	OrcaOwner    port.ExecutionOrcaOwnerInspector
	ReadIssue    executionissue.ExecutionIssueSnapshotReadFunc
	Claim        issueopscontract.ExecutionClaimHandler
	Release      issueopscontract.ExecutionReleaseHandler
	Reseed       issueopscontract.ExecutionReseedHandler
	Resume       issueopscontract.ExecutionResumeHandler
	Reconcile    port.ExecutionReconcileHandler
	Complete     issueopscontract.ExecutionCompleteHandler
	Publication  PublicationHandlers
	Provenance   provenanceport.Observer
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

func HandleToolCallWithDependencies(params json.RawMessage, deps MCPDependencies) (any, *jsonrpc.Error) {
	var call MCPToolCall
	if err := json.Unmarshal(params, &call); err != nil {
		return nil, newProtocolError(-32602, "Invalid params", err.Error())
	}
	if call.Arguments == nil {
		call.Arguments = map[string]any{}
	}
	if validationErr := validateMCPToolArguments(deps.Catalog, call.Name, call.Arguments); validationErr != nil {
		return nil, validationErr
	}
	for _, handler := range []func(MCPToolCall) MCPToolOutcome{
		func(call MCPToolCall) MCPToolOutcome { return handleProjectMCPToolCall(call, deps.ProjectDocs) },
		func(call MCPToolCall) MCPToolOutcome { return handlePolicyStateMCPToolCall(call, deps.State) },
		func(call MCPToolCall) MCPToolOutcome {
			return handleIssueOpsMCPToolCallWithDependencies(call, deps)
		},
		handleLoopMCPToolCall,
		handleGatesMCPToolCall,
		handleChannelMCPToolCall,
		handleAssistantWorkerMCPToolCall,
		func(call MCPToolCall) MCPToolOutcome { return handleSelfLoopMCPToolCall(call, deps) },
	} {
		outcome := handler(call)
		if !outcome.Handled {
			continue
		}
		if outcome.Err != nil {
			return nil, outcome.Err
		}
		if outcome.Direct {
			return outcome.Result, nil
		}
		b, _ := json.MarshalIndent(outcome.Payload, "", "  ")
		if outcome.IsError {
			return ErrorTextResult(string(b)), nil
		}
		return TextResult(string(b)), nil
	}
	return nil, newProtocolError(-32602, "Unknown tool", call.Name)
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
