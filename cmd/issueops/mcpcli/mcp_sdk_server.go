package mcpcli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"issueops/cmd/issueops/mcpcli/resources"
	mcpcontract "issueops/internal/contract/mcp"
)

func initSDKServer(deps MCPDependencies) *mcp.Server {
	return initSDKServerWithDiagnostics(deps, io.Discard)
}

func initSDKServerWithDiagnostics(deps MCPDependencies, diagnostics io.Writer) *mcp.Server {
	if diagnostics == nil {
		diagnostics = io.Discard
	}
	return newSDKServer(deps, sdkServerOptionsWithDiagnostics(diagnostics), transportStdio, nil)
}

const httpServerInstructions = "This MCP endpoint is the shared local service used by every host session, not a per-host process. Workspace tools need an authority_file issued by 'issueops mcp authorize'. Use harness tools for shared Codex/Claude inspection, atomic commit preflight, state checkpoints, self-verification, self-augmentation, and commit policy context. External wiki or knowledge-base workflows belong to their own separately installed servers, not issueops."

func newSDKServer(deps MCPDependencies, options *mcp.ServerOptions, transport serverTransport, access *slog.Logger) *mcp.Server {
	if transport == transportHTTP {
		options.Instructions = httpServerInstructions
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "issueops", Version: deps.Resources.Version}, options)
	registerAllTools(server, deps, transport, access)
	registerAllResources(server, deps)
	return server
}

// initSDKServerWithLogger는 진단 writer 대신 로거를 직접 받는 변형이다.
// 데몬 경로가 세션 루틴 이벤트를 DEBUG로 강등하는 필터 로거를 넘긴다.
func initSDKServerWithLogger(deps MCPDependencies, logger *slog.Logger) *mcp.Server {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return newSDKServer(deps, sdkServerOptionsWithLogger(logger), transportStdio, nil)
}

func sdkServerOptions() *mcp.ServerOptions {
	return sdkServerOptionsWithDiagnostics(io.Discard)
}

func sdkServerOptionsWithDiagnostics(diagnostics io.Writer) *mcp.ServerOptions {
	return sdkServerOptionsWithLogger(slog.New(slog.NewTextHandler(diagnostics, nil)))
}

func sdkServerOptionsWithLogger(logger *slog.Logger) *mcp.ServerOptions {
	return &mcp.ServerOptions{
		Instructions: "This MCP endpoint runs the issueops harness in-process for the calling host session. Use harness tools for shared Codex/Claude inspection, atomic commit preflight, state checkpoints, self-verification, self-augmentation, and commit policy context. External wiki or knowledge-base workflows belong to their own separately installed servers, not issueops.",
		Logger:       logger,
		Capabilities: &mcp.ServerCapabilities{},
	}
}

func sdkToolHandlerWithContext(
	catalog mcpcontract.Catalog,
	groupHandler func(context.Context, MCPToolCall) MCPToolOutcome,
	toolName string,
) mcp.ToolHandler {
	output, outputErr := compileToolOutputSchema(catalog, toolName)
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, newProtocolError(-32602, "invalid arguments", err.Error())
			}
		}
		if args == nil {
			args = map[string]any{}
		}
		if validationErr := validateMCPToolArguments(catalog, toolName, args); validationErr != nil {
			return nil, validationErr
		}
		outcome := groupHandler(ctx, MCPToolCall{Name: toolName, Arguments: args})
		if outcome.Err != nil {
			return nil, outcome.Err
		}
		if outcome.Direct {
			if dm, ok := outcome.Result.(map[string]any); ok {
				var content []map[string]any
				switch items := dm["content"].(type) {
				case []map[string]any:
					content = items
				case []any:
					content = make([]map[string]any, 0, len(items))
					for _, item := range items {
						if cm, ok := item.(map[string]any); ok {
							content = append(content, cm)
						}
					}
				}
				if content != nil {
					isError, _ := dm["isError"].(bool)
					result := &mcp.CallToolResult{IsError: isError}
					for _, cm := range content {
						if text, ok := cm["text"].(string); ok && cm["type"] == "text" {
							result.Content = append(result.Content, &mcp.TextContent{Text: text})
						}
					}
					return result, nil
				}
			}
			b, _ := json.MarshalIndent(outcome.Result, "", "  ")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}}, nil
		}
		b, _ := json.MarshalIndent(outcome.Payload, "", "  ")
		result := &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
			IsError: outcome.IsError,
		}
		if output != nil || outputErr != nil {
			if outcome.IsError {
				return result, nil
			}
			structured, err := validatedStructuredContent(output, outputErr, outcome.Payload)
			if err != nil {
				return nil, newProtocolError(-32603, "Invalid tool output", toolName)
			}
			result.StructuredContent = structured
		}
		return result, nil
	}
}

// compileToolOutputSchema resolves the catalog outputSchema of a tool once, so
// every call validates against the same compiled schema. A tool without an
// outputSchema returns nil.
func compileToolOutputSchema(catalog mcpcontract.Catalog, name string) (*jsonschema.Resolved, error) {
	for _, tool := range catalog.Tools {
		if toolName, _ := tool["name"].(string); toolName != name {
			continue
		}
		raw, ok := tool["outputSchema"].(map[string]any)
		if !ok {
			return nil, nil
		}
		encoded, err := json.Marshal(raw)
		if err != nil {
			return nil, err
		}
		var schema jsonschema.Schema
		if err := json.Unmarshal(encoded, &schema); err != nil {
			return nil, err
		}
		return schema.Resolve(nil)
	}
	return nil, nil
}

// validatedStructuredContent returns the payload as the JSON object that
// structuredContent carries, after checking it against the tool's outputSchema.
// Errors never include the payload.
func validatedStructuredContent(schema *jsonschema.Resolved, schemaErr error, payload any) (any, error) {
	if schemaErr != nil {
		return nil, schemaErr
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var structured any
	if err := json.Unmarshal(encoded, &structured); err != nil {
		return nil, err
	}
	if err := schema.Validate(structured); err != nil {
		return nil, errors.New("output does not match outputSchema")
	}
	return structured, nil
}

func registerAllTools(server *mcp.Server, deps MCPDependencies, transport serverTransport, access *slog.Logger) {
	for _, toolMap := range deps.Catalog.Tools {
		name, _ := toolMap["name"].(string)
		desc, _ := toolMap["description"].(string)
		inputSchema := toolMap["inputSchema"]
		sdkTool := &mcp.Tool{Name: name, Description: desc, InputSchema: inputSchema}
		if outputSchema, ok := toolMap["outputSchema"].(map[string]any); ok {
			sdkTool.OutputSchema = outputSchema
		}
		if annotations, ok := toolMap["annotations"].(*mcpcontract.ToolAnnotations); ok {
			sdkTool.Annotations = sdkToolAnnotations(annotations)
		}
		handler := sdkToolHandlerWithContext(deps.Catalog, func(ctx context.Context, call MCPToolCall) MCPToolOutcome {
			return dispatchMCPToolCall(ctx, call, deps, transport)
		}, name)
		if transport == transportHTTP {
			handler = httpToolHandler(handler, name, deps.BindTrace, access)
		}
		server.AddTool(sdkTool, handler)
	}
}

// sdkToolAnnotations converts the catalog hints. The SDK serializes its
// non-pointer hints even when false, so a tool without annotations keeps nil.
func sdkToolAnnotations(annotations *mcpcontract.ToolAnnotations) *mcp.ToolAnnotations {
	out := &mcp.ToolAnnotations{DestructiveHint: annotations.DestructiveHint, OpenWorldHint: annotations.OpenWorldHint}
	if annotations.ReadOnlyHint != nil {
		out.ReadOnlyHint = *annotations.ReadOnlyHint
	}
	if annotations.IdempotentHint != nil {
		out.IdempotentHint = *annotations.IdempotentHint
	}
	return out
}

func resolveHandlerGroup(deps MCPDependencies, name string) func(context.Context, MCPToolCall) MCPToolOutcome {
	switch deps.Catalog.Dispatch[name] {
	case mcpcontract.DispatchAssistantWorker:
		return func(ctx context.Context, call MCPToolCall) MCPToolOutcome {
			return handleAssistantWorkerMCPToolCall(ctx, call, deps)
		}
	case mcpcontract.DispatchGates:
		return func(_ context.Context, call MCPToolCall) MCPToolOutcome {
			return handleGatesMCPToolCall(call, deps.Gates)
		}
	case mcpcontract.DispatchChannel:
		return func(_ context.Context, call MCPToolCall) MCPToolOutcome {
			return handleChannelMCPToolCall(call, deps.Channel)
		}
	case mcpcontract.DispatchLoop:
		return func(ctx context.Context, call MCPToolCall) MCPToolOutcome {
			return handleLoopMCPToolCall(ctx, call, deps.Loop)
		}
	case mcpcontract.DispatchProject:
		return func(_ context.Context, call MCPToolCall) MCPToolOutcome { return handleProjectMCPToolCall(call, deps) }
	case mcpcontract.DispatchPolicyState:
		return func(ctx context.Context, call MCPToolCall) MCPToolOutcome {
			return handlePolicyStateMCPToolCall(ctx, call, deps)
		}
	case mcpcontract.DispatchSelfLoop:
		return func(ctx context.Context, call MCPToolCall) MCPToolOutcome {
			return handleSelfLoopMCPToolCall(ctx, call, deps)
		}
	case mcpcontract.DispatchIssueOps:
		return func(ctx context.Context, call MCPToolCall) MCPToolOutcome {
			return handleIssueOpsMCPToolCallWithContext(ctx, call, deps)
		}
	default:
		return func(_ context.Context, call MCPToolCall) MCPToolOutcome {
			return MCPToolOutcome{Handled: true, Err: newProtocolError(-32602, "Unknown tool", call.Name)}
		}
	}
}

func HandleResourceRead(params json.RawMessage, config resources.Config) (any, *jsonrpc.Error) {
	result, readErr := resources.HandleResourceRead(params, config)
	if readErr != nil {
		return nil, newProtocolError(int64(readErr.Code), readErr.Message, readErr.Data)
	}
	return result, nil
}

func registerAllResources(server *mcp.Server, deps MCPDependencies) {
	for _, r := range deps.Catalog.Resources {
		uri, _ := r["uri"].(string)
		name, _ := r["name"].(string)
		desc, _ := r["description"].(string)
		mime, _ := r["mimeType"].(string)
		server.AddResource(
			&mcp.Resource{URI: uri, Name: name, Description: desc, MIMEType: mime},
			sdkResourceHandler(deps.Resources),
		)
	}
}

func sdkResourceHandler(config resources.Config) mcp.ResourceHandler {
	return func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		params, err := json.Marshal(map[string]string{"uri": req.Params.URI})
		if err != nil {
			return nil, fmt.Errorf("marshal resource params: %w", err)
		}
		result, readErr := HandleResourceRead(params, config)
		if readErr != nil {
			return nil, readErr
		}
		if m, ok := result.(map[string]any); ok {
			if contents, ok := m["contents"].([]any); ok && len(contents) > 0 {
				if cm, ok := contents[0].(map[string]any); ok {
					return sdkReadResourceResult(cm), nil
				}
			}
			if contents, ok := m["contents"].([]map[string]any); ok && len(contents) > 0 {
				return sdkReadResourceResult(contents[0]), nil
			}
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, MIMEType: "application/json", Text: string(b)}},
		}, nil
	}
}

func sdkReadResourceResult(content map[string]any) *mcp.ReadResourceResult {
	uri, _ := content["uri"].(string)
	mimeType, _ := content["mimeType"].(string)
	text, _ := content["text"].(string)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: text}},
	}
}

// serveMCPStreamSDK runs both split stdio and bidirectional daemon connections
// through the official go-sdk IOTransport.
func serveMCPStreamSDK(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	server := initSDKServerWithDiagnostics(deps, diagnostics)
	if rwc, ok := input.(io.ReadWriteCloser); ok && io.Writer(rwc) == output {
		return server.Run(ctx, &mcp.IOTransport{Reader: rwc, Writer: rwc})
	}
	// 분리된 stdio는 host가 stdin을 닫아도 이미 받은 요청에 끝까지 응답한다.
	inflight := newInflightRequests()
	var closer io.Closer
	if inputCloser, ok := input.(io.Closer); ok {
		closer = inputCloser
	}
	reader := &drainingReader{source: bufio.NewReader(input), closer: closer, inflight: inflight}
	writer := &observingWriter{target: output, inflight: inflight}
	return server.Run(ctx, &mcp.IOTransport{Reader: reader, Writer: writer})
}

type writeCloser struct{ io.Writer }

func (writeCloser) Close() error { return nil }
