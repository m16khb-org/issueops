package issueopsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/webfetchcli"
)

type webFetchRoundTripper func(*http.Request) (*http.Response, error)

func (transport webFetchRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return transport(req)
}

func TestWebFetchCLIAndMCPKeepCapturedHTTPClients(t *testing.T) {
	previous := http.DefaultClient
	t.Cleanup(func() { http.DefaultClient = previous })
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	type prepared struct {
		cli     webfetchcli.Deps
		mcp     mcpcli.MCPDependencies
		session *mcp.ClientSession
		name    string
	}
	var commands []prepared
	for _, name := range []string{"first", "second"} {
		http.DefaultClient = &http.Client{Transport: webFetchRoundTripper(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host != "8.8.8.8" {
				t.Fatalf("unexpected network target: %s", req.URL)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"owner":"%s"}`, name)))}, nil
		})}
		cli, deps := webFetchDependencies(), issueOpsMCPDependencies()
		commands = append(commands, prepared{cli: cli, mcp: deps, session: startHistoryMCPTestSession(t, deps), name: name})
	}
	http.DefaultClient = &http.Client{Transport: webFetchRoundTripper(func(*http.Request) (*http.Response, error) {
		t.Fatal("ambient HTTP client used after composition")
		return nil, nil
	})}
	for _, command := range commands {
		var out bytes.Buffer
		deps := command.cli
		deps.Stdout = &out
		if err := webfetchcli.RunWithDeps([]string{"fetch", "--url", "http://8.8.8.8", "--json"}, deps); err != nil || !strings.Contains(out.String(), command.name) {
			t.Fatalf("CLI lost %s: %s %v", command.name, out.String(), err)
		}
		call := mcpcli.MCPToolCall{Name: "web_fetch_resilient", Arguments: map[string]any{"url": "http://8.8.8.8"}}
		raw, _ := json.Marshal(call)
		result, err := callSDKTool(t, raw, command.mcp)
		encoded, _ := json.Marshal(result)
		if err != nil || !bytes.Contains(encoded, []byte(command.name)) {
			t.Fatalf("direct MCP lost %s: %s %v", command.name, encoded, err)
		}
		sdk, sdkErr := command.session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.Name, Arguments: call.Arguments})
		if sdkErr != nil {
			t.Fatal(sdkErr)
		}
		encoded, _ = json.Marshal(sdk)
		if sdk.IsError || !bytes.Contains(encoded, []byte(command.name)) {
			t.Fatalf("SDK MCP lost %s: %s", command.name, encoded)
		}
	}
}
