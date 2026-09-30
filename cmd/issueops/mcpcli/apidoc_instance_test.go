package mcpcli

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	app "issueops/internal/application/apidoc"
)

func apiDocOutcomeDeps(t *testing.T, owner, mode string) MCPDependencies {
	t.Helper()
	service := testAPIDocService()
	normalize := func(repo string, _ []string) []string {
		if repo != owner {
			t.Errorf("%s reader received %s", owner, repo)
		}
		return []string{owner + ".controller.ts"}
	}
	service.Static.Effects.NormalizeFiles = normalize
	service.Static.Effects.Mode = func(string) string {
		if mode == "static-fail" || mode == "static-io" {
			return ""
		}
		return "contract-tests"
	}
	service.Static.Effects.ReadFile = func(string, string) (string, error) {
		if mode == "static-io" {
			return "", errors.New(owner + "-unreadable")
		}
		return "@Controller('x')\nexport class X {\n @Get()\n list() {}\n}", nil
	}
	service.Reviewer.Effects.NormalizeFiles = normalize
	service.Reviewer.Effects.Input = func(string, []string, string, bool) (string, error) {
		if mode == "review-io" {
			return "", errors.New(owner + "-unreadable")
		}
		return owner + "-content", nil
	}
	service.Reviewer.Effects.ExtraPrompt = func(app.ReviewOptions) (string, error) { return owner + "-instructions", nil }
	service.Reviewer.Effects.Evidence = func(string, []string) string { return owner + "-evidence" }
	service.Reviewer.Effects.ReadResult = func(string, string) (string, []byte, error) {
		return owner + "-result", []byte(`{"verdict":"fail"}`), nil
	}
	return MCPDependencies{Catalog: testMCPCatalog(), DefaultTarget: owner, APIDoc: service}
}

func TestAPIDocDirectAndSDKKeepServicesAndGateErrors(t *testing.T) {
	for _, mode := range []string{"pending", "review-fail", "static-fail", "review-io", "static-io"} {
		t.Run(mode, func(t *testing.T) {
			owners := []string{"first-api", "second-api"}
			deps := []MCPDependencies{apiDocOutcomeDeps(t, owners[0], mode), apiDocOutcomeDeps(t, owners[1], mode)}
			sessions := []*mcp.ClientSession{startMCPTransportTestSession(t, "stdio", deps[0]), startMCPTransportTestSession(t, "stdio", deps[1])}
			tool := "api_doc_review"
			if strings.HasPrefix(mode, "static") {
				tool = "api_doc_static_check"
			}
			args := map[string]any{}
			if mode == "review-fail" {
				args["result_file"] = "result.json"
			}
			raw, _ := json.Marshal(map[string]any{"name": tool, "arguments": args})
			for _, i := range []int{0, 1, 0} {
				direct, rpcErr := HandleToolCallWithDependencies(raw, deps[i])
				sdk, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
				failedIO := strings.HasSuffix(mode, "-io")
				var sdkText string
				if failedIO {
					var protocolErr *jsonrpc.Error
					if rpcErr == nil || rpcErr.Code != -32000 || !errors.As(err, &protocolErr) || protocolErr.Code != -32000 {
						t.Fatalf("%s direct=%v sdk=%v", mode, rpcErr, err)
					}
					encoded, _ := json.Marshal(protocolErr)
					sdkText = string(encoded)
				} else {
					if rpcErr != nil || err != nil || sdk.IsError {
						t.Fatalf("%s direct=%v sdk=%+v err=%v", mode, rpcErr, sdk, err)
					}
					sdkText = toolResultText(sdk)
				}
				var text string
				if rpcErr != nil {
					bytes, _ := json.Marshal(rpcErr)
					text = string(bytes)
				} else {
					text = extractSingleTextResult(t, direct)
				}
				for _, output := range []string{text, sdkText} {
					if !strings.Contains(output, owners[i]) || strings.Contains(output, owners[1-i]) {
						t.Fatalf("%s wrong instance: %s", mode, output)
					}
					if mode == "pending" && !strings.Contains(output, "host_agent_result_required") {
						t.Fatalf("pending gate lost: %s", output)
					}
					if mode == "static-fail" && !strings.Contains(output, "missing_api_operation") {
						t.Fatalf("static gate lost: %s", output)
					}
				}
			}
		})
	}
}
