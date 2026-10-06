package mcpcli

import (
	"context"
	"encoding/json"
	"fmt"
	issueopscontract "issueops/internal/contract/issueops"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSDKToolHandlerDispatchesCatalogTool(t *testing.T) {
	handler := sdkToolHandlerWithContext(testMCPCatalog(), resolveHandlerGroup(testTransportServices(), "contract_schema"), "contract_schema")
	result, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{}`)},
	})
	if err != nil {
		t.Fatalf("sdk tool handler returned error: %v", err)
	}
	if result == nil || len(result.Content) != 1 {
		t.Fatalf("sdk tool handler result = %#v, want one text content", result)
	}
	text, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("sdk tool content type = %T, want *mcp.TextContent", result.Content[0])
	}
	if !strings.Contains(text.Text, "issueops_cli_mcp_compatibility") {
		t.Fatalf("sdk tool response missing contract schema payload:\n%s", text.Text)
	}
}

func TestSDKDirectContentPreservesTextAndError(t *testing.T) {
	for _, shape := range []string{"maps", "any"} {
		for _, isError := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/error=%t", shape, isError), func(t *testing.T) {
				const text = "# Policy\n\nexact bytes\t\n"
				var content any = []map[string]any{{"type": "text", "text": text}}
				if shape == "any" {
					content = []any{map[string]any{"type": "text", "text": text}}
				}
				server := initSDKServer(MCPDependencies{})
				server.AddTool(&mcp.Tool{Name: "commit_policy", InputSchema: map[string]any{"type": "object"}},
					sdkToolHandlerWithContext(testMCPCatalog(), func(context.Context, MCPToolCall) MCPToolOutcome {
						return mcpToolDirect(map[string]any{"content": content, "isError": isError})
					}, "commit_policy"))
				serverTransport, clientTransport := mcp.NewInMemoryTransports()
				serverSession, err := server.Connect(t.Context(), serverTransport, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer serverSession.Close()
				client := mcp.NewClient(&mcp.Implementation{Name: "direct-content-test", Version: "1"}, nil)
				session, err := client.Connect(t.Context(), clientTransport, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer session.Close()
				result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "commit_policy", Arguments: map[string]any{}})
				if err != nil || result.IsError != isError || len(result.Content) != 1 || toolResultText(result) != text {
					t.Fatalf("direct result lost text/error: result=%+v err=%v", result, err)
				}
			})
		}
	}
}

func TestSDKCommitPolicyReturnsExactResourceText(t *testing.T) {
	deps := testTransportServices()
	want, err := deps.Resources.ReadHarnessFile(".issueops", "COMMIT_POLICY.md")
	if err != nil {
		t.Fatal(err)
	}
	session := startMCPTransportTestSession(t, "stdio", deps)
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "commit_policy", Arguments: map[string]any{}})
	if err != nil || result.IsError || len(result.Content) != 1 || toolResultText(result) != want {
		t.Fatalf("commit_policy must return exact Markdown: result=%+v err=%v", result, err)
	}
}

func TestSDKToolHandlerRejectsInvalidRawArguments(t *testing.T) {
	handler := sdkToolHandlerWithContext(testMCPCatalog(), resolveHandlerGroup(testTransportServices(), "contract_schema"), "contract_schema")
	_, err := handler(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(`{not json`)},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid arguments") {
		t.Fatalf("sdk tool invalid args error = %v, want invalid arguments", err)
	}
}

func TestSDKResourceHandlerReadsHarnessResource(t *testing.T) {
	result, err := sdkResourceHandler(resourceConfigForTest())(context.Background(), &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{URI: "issueops://commit-policy"},
	})
	if err != nil {
		t.Fatalf("sdk resource handler returned error: %v", err)
	}
	if result == nil || len(result.Contents) != 1 {
		t.Fatalf("sdk resource result = %#v, want one resource content", result)
	}
	content := result.Contents[0]
	if content.URI != "issueops://commit-policy" || content.MIMEType != "text/markdown" {
		t.Fatalf("unexpected sdk resource content metadata: %#v", content)
	}
	if !strings.Contains(strings.ToLower(content.Text), "commit") {
		t.Fatalf("sdk resource content did not include commit policy text:\n%s", content.Text)
	}
}

func TestInitSDKServerKeepsDependenciesPerServer(t *testing.T) {
	first := initSDKServer(testTransportServices())
	second := initSDKServer(testTransportServices())

	if first == nil || second == nil {
		t.Fatalf("initSDKServer returned nil: first=%v second=%v", first, second)
	}
	if first == second {
		t.Fatal("initSDKServer must not cache a package-global server")
	}
}

func TestInitSDKServerAcceptsPublicationReconcileWithoutInvokingIt(t *testing.T) {
	invoked := 0
	handler := issueopscontract.RemotePullRequestReconcileHandler(func(context.Context, string, issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error) {
		invoked++
		return issueopscontract.ExecutionReconcileResult{}, nil
	})

	server := initSDKServer(MCPDependencies{Execution: testExecutionDeps(), Catalog: testMCPCatalog(), Publication: PublicationHandlers{Reconcile: handler}})
	if server == nil {
		t.Fatal("initSDKServer returned nil")
	}
	if invoked != 0 {
		t.Fatalf("SDK registration invoked publication reconcile handler: %d", invoked)
	}
}

func TestInitSDKServerDispatchesConcurrentReleaseWithIsolatedDependencies(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	ids := []string{"io-sdk-first", "io-sdk-second"}
	called := make(chan string, len(ids))
	errs := make(chan error, len(ids))
	var group sync.WaitGroup
	for _, id := range ids {
		group.Add(1)
		go func(id string) {
			defer group.Done()
			server := initSDKServer(MCPDependencies{Execution: testExecutionDeps(), Catalog: testMCPCatalog(), Release: func(_ context.Context, _ string, request issueopscontract.ExecutionReleaseRequest) (issueopscontract.ExecutionResult, error) {
				token := id + "::" + request.ID
				called <- token
				return issueopscontract.ExecutionResult{OK: true, ID: token}, nil
			}})
			clientTransport, serverTransport := mcp.NewInMemoryTransports()
			serverSession, err := server.Connect(context.Background(), serverTransport, nil)
			if err != nil {
				errs <- err
				return
			}
			defer serverSession.Close()
			client := mcp.NewClient(&mcp.Implementation{Name: "issueops-sdk-test", Version: "1"}, nil)
			clientSession, err := client.Connect(context.Background(), clientTransport, nil)
			if err != nil {
				errs <- err
				return
			}
			defer clientSession.Close()
			result, err := clientSession.CallTool(context.Background(), &mcp.CallToolParams{Name: "issueops_execution", Arguments: map[string]any{"action": "release", "id": id, "generation": 1}})
			if err != nil {
				errs <- err
				return
			}
			if result == nil || len(result.Content) != 1 {
				errs <- fmt.Errorf("SDK release handler returned no content for %s", id)
				return
			}
			text, ok := result.Content[0].(*mcp.TextContent)
			if !ok || !strings.Contains(text.Text, id+"::"+id) {
				errs <- fmt.Errorf("SDK release response did not bind handler and request for %s: %#v", id, result.Content[0])
			}
		}(id)
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	close(called)
	seen := map[string]bool{}
	for token := range called {
		seen[token] = true
	}
	for _, id := range ids {
		if !seen[id+"::"+id] {
			t.Fatalf("SDK release handler/request binding for %s was not observed", id)
		}
	}
}

func TestSDKServerOptionsDoNotAdvertiseUnusedLoggingState(t *testing.T) {
	options := sdkServerOptions()
	if options.Capabilities == nil {
		t.Fatal("SDK server must use explicit capabilities instead of logging defaults")
	}
	if _, present := sdkCapabilityFields(t, options.Capabilities)["logging"]; present {
		t.Fatalf("unused logging capability must remain disabled: %#v", options.Capabilities)
	}
}

func TestSDKServerHandshakeOmitsLoggingAndKeepsCatalogCapabilities(t *testing.T) {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "issueops_test", Version: "0"},
		sdkServerOptions(),
	)
	registerAllTools(server, testTransportServices(), transportStdio, nil)
	registerAllResources(server, MCPDependencies{Execution: testExecutionDeps(), Catalog: testMCPCatalog(), Resources: resourceConfigForTest()})
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	capabilities := clientSession.InitializeResult().Capabilities
	fields := sdkCapabilityFields(t, capabilities)
	if _, present := fields["logging"]; present || capabilities.Tools == nil || capabilities.Resources == nil {
		t.Fatalf("SDK handshake capabilities = %#v", capabilities)
	}
}

func sdkCapabilityFields(t *testing.T, capabilities *mcp.ServerCapabilities) map[string]json.RawMessage {
	t.Helper()
	body, err := json.Marshal(capabilities)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}
