package mcpcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	model "issueops/internal/contract/issueops"
)

const revisionMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"revision-test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`

type revisionMessage struct {
	ID     int             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  *jsonrpc.Error  `json:"error"`
}

type revisionStdio struct {
	t       *testing.T
	ctx     context.Context
	encoder *json.Encoder
	decoder *json.Decoder
}

func startRevisionStdio(t *testing.T, deps MCPDependencies) *revisionStdio {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	serverReader, clientWriter := io.Pipe()
	clientReader, serverWriter := io.Pipe()
	closePipes := func() {
		for _, pipe := range []io.Closer{serverReader, clientWriter, clientReader, serverWriter} {
			if err := pipe.Close(); err != nil {
				t.Errorf("close stdio pipe: %v", err)
			}
		}
	}
	stop := context.AfterFunc(ctx, closePipes)
	done := make(chan error, 1)
	go func() {
		done <- ServeMCPStreamContextWithDependencies(ctx, serverReader, serverWriter, io.Discard, deps)
	}()
	t.Cleanup(func() {
		cancel()
		if stop() {
			closePipes()
		}
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.ErrClosedPipe) {
				t.Errorf("stdio shutdown: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("stdio server did not stop after pipe closure")
		}
	})
	return &revisionStdio{t: t, ctx: ctx, encoder: json.NewEncoder(clientWriter), decoder: json.NewDecoder(clientReader)}
}

func (p *revisionStdio) send(message string) {
	p.t.Helper()
	if err := p.encoder.Encode(json.RawMessage(message)); err != nil {
		p.t.Fatalf("write stdio message: %v", err)
	}
}

func (p *revisionStdio) read() revisionMessage {
	p.t.Helper()
	var message revisionMessage
	if err := p.decoder.Decode(&message); err != nil {
		p.t.Fatalf("read stdio message: %v", err)
	}
	if err := p.ctx.Err(); err != nil {
		p.t.Fatalf("stdio response arrived after test cancellation: %v", err)
	}
	return message
}

func (p *revisionStdio) initialize(revision string) mcp.InitializeResult {
	p.t.Helper()
	p.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":%q,"capabilities":{},"clientInfo":{"name":"revision-test","version":"1"}}}`, revision))
	message := p.read()
	var result mcp.InitializeResult
	if message.ID != 1 || message.Error != nil {
		p.t.Fatalf("initialize response: %+v", message)
	}
	if err := json.Unmarshal(message.Result, &result); err != nil {
		p.t.Fatal(err)
	}
	p.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	return result
}

func TestMCPRevisionStdioInitializeNegotiation(t *testing.T) {
	for _, tc := range []struct{ requested, negotiated string }{
		{"2025-06-18", "2025-06-18"},
		{"2025-11-25", "2025-11-25"},
		{"2026-07-28", "2025-11-25"},
		{"2099-01-01", "2025-11-25"},
	} {
		t.Run(tc.requested, func(t *testing.T) {
			// Given: the production split stdio entry point.
			peer := startRevisionStdio(t, testTransportServices())
			// When: an initialize-based client negotiates a revision.
			result := peer.initialize(tc.requested)
			// Then: initialize never negotiates into the sessionless protocol.
			if result.ProtocolVersion != tc.negotiated {
				t.Fatalf("negotiated %q, want %q", result.ProtocolVersion, tc.negotiated)
			}
			peer.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
			assertRevisionTools(t, peer.read())
		})
	}
}

func TestMCPRevisionStdioRequestMetadata(t *testing.T) {
	for _, tc := range []struct {
		name, method, params string
		code                 int64
	}{
		{"discover", "server/discover", `{` + revisionMeta + `}`, 0},
		{"list_without_initialize", "tools/list", `{` + revisionMeta + `}`, 0},
		{"call_without_initialize", "tools/call", `{"name":"contract_schema","arguments":{},` + revisionMeta + `}`, 0},
		{"optional_client_info", "tools/list", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}`, 0},
		{"unsupported_revision", "tools/list", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2099-01-01","io.modelcontextprotocol/clientCapabilities":{}}}`, -32022},
		{"missing_capabilities", "tools/list", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}`, -32602},
		{"invalid_client_info", "tools/list", `{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{},"io.modelcontextprotocol/clientInfo":7}}`, -32602},
		{"discover_without_metadata", "server/discover", `{}`, -32601},
		{"removed_ping", "ping", `{` + revisionMeta + `}`, -32601},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Given: a fresh stdio connection without an initialize handshake.
			peer := startRevisionStdio(t, testTransportServices())
			// When: the request selects its own protocol via metadata.
			peer.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":%q,"params":%s}`, tc.method, tc.params))
			// Then: revision selection and metadata validation are observable on the wire.
			message := peer.read()
			if message.ID != 2 {
				t.Fatalf("response ID = %d, want 2", message.ID)
			}
			if tc.code != 0 {
				if message.Error == nil || message.Error.Code != tc.code {
					t.Fatalf("error = %+v, want code %d", message.Error, tc.code)
				}
				if tc.code == -32022 {
					var data struct {
						Requested string   `json:"requested"`
						Supported []string `json:"supported"`
					}
					if err := json.Unmarshal(message.Error.Data, &data); err != nil {
						t.Fatal(err)
					}
					if data.Requested != "2099-01-01" || !slices.Contains(data.Supported, "2026-07-28") || !slices.Contains(data.Supported, "2025-11-25") {
						t.Fatalf("unsupported revision data = %+v", data)
					}
				}
				return
			}
			if message.Error != nil {
				t.Fatalf("request error: %v", message.Error)
			}
			var result struct {
				ResultType        string         `json:"resultType"`
				SupportedVersions []string       `json:"supportedVersions"`
				Meta              map[string]any `json:"_meta"`
			}
			if err := json.Unmarshal(message.Result, &result); err != nil {
				t.Fatal(err)
			}
			if result.ResultType != "complete" || result.Meta["io.modelcontextprotocol/serverInfo"] == nil {
				t.Fatalf("new-protocol response resultType = %q, server identity = %v", result.ResultType, result.Meta["io.modelcontextprotocol/serverInfo"])
			}
			switch tc.method {
			case "server/discover":
				if !slices.Contains(result.SupportedVersions, "2026-07-28") || !slices.Contains(result.SupportedVersions, "2025-11-25") {
					t.Fatalf("discover versions = %v", result.SupportedVersions)
				}
			case "tools/list":
				assertRevisionTools(t, message)
			case "tools/call":
				var call mcp.CallToolResult
				if err := json.Unmarshal(message.Result, &call); err != nil {
					t.Fatal(err)
				}
				var payload struct {
					OK bool `json:"ok"`
				}
				if err := json.Unmarshal([]byte(toolResultText(&call)), &payload); err != nil || call.IsError || !payload.OK {
					t.Fatalf("contract tool response = %s, parse error = %v", message.Result, err)
				}
			}
		})
	}
}

func assertRevisionTools(t *testing.T, message revisionMessage) {
	t.Helper()
	var result mcp.ListToolsResult
	if err := json.Unmarshal(message.Result, &result); err != nil || message.Error != nil || !containsSDKTool(result.Tools, "docs_index") {
		t.Fatalf("tool catalog response = %+v, parse error = %v", message, err)
	}
}

func TestMCPRevisionStdioCancellation(t *testing.T) {
	for _, revision := range []string{"2025-11-25", "2026-07-28"} {
		t.Run(revision, func(t *testing.T) {
			// Given: subscribe to handler entry and cancellation before sending the call.
			entered, cancelled := make(chan struct{}), make(chan struct{})
			deps := testTransportServices()
			deps.Release = func(ctx context.Context, _ string, _ model.ExecutionReleaseRequest) (model.ExecutionResult, error) {
				close(entered)
				<-ctx.Done()
				close(cancelled)
				return model.ExecutionResult{}, ctx.Err()
			}
			peer := startRevisionStdio(t, deps)
			meta := ""
			if revision == "2025-11-25" {
				peer.initialize(revision)
			} else {
				meta = "," + revisionMeta
			}
			peer.send(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"issueops_execution","arguments":{"action":"release","id":"io-revision","generation":1}` + meta + `}}`)
			select {
			case <-entered:
			case <-peer.ctx.Done():
				t.Fatal("execution handler did not start")
			}
			// When: either revision cancels the exact in-flight stdio request.
			peer.send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":7}}`)
			// Then: the production execution dispatch propagates context cancellation.
			select {
			case <-cancelled:
				if err := peer.ctx.Err(); err != nil {
					t.Fatalf("handler cancelled by test timeout instead of notification: %v", err)
				}
			case <-peer.ctx.Done():
				t.Fatal("execution handler did not receive cancellation")
			}
		})
	}
}

func TestMCPRevisionStdioSubscriptionCancellation(t *testing.T) {
	// Given: a fresh production stdio connection.
	peer := startRevisionStdio(t, testTransportServices())
	// When: subscribe and observe acknowledgement before cancelling the listen.
	peer.send(`{"jsonrpc":"2.0","id":7,"method":"subscriptions/listen","params":{"notifications":{"toolsListChanged":true},` + revisionMeta + `}}`)
	message := peer.read()
	var ack struct {
		Notifications struct {
			ToolsListChanged bool `json:"toolsListChanged"`
		} `json:"notifications"`
	}
	if err := json.Unmarshal(message.Params, &ack); err != nil || message.Method != "notifications/subscriptions/acknowledged" || !ack.Notifications.ToolsListChanged {
		t.Fatalf("subscription acknowledgement = %+v, parse error = %v", message, err)
	}
	peer.send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":7}}`)
	// Then: explicit stdio cancellation completes the acknowledged listen.
	message = peer.read()
	if message.ID != 7 || message.Error != nil || len(message.Result) == 0 {
		t.Fatalf("subscription completion = %+v", message)
	}
}
