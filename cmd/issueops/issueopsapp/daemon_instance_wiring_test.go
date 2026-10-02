package issueopsapp

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	contract "issueops/internal/contract/daemon"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDaemonMCPInstancesKeepCapturedPathsAndCapacity(t *testing.T) {
	var deps [2]mcpcli.MCPDependencies
	var clients [2]*mcp.ClientSession
	var dirs [2]string
	limits := []int{3, 7}
	for i := range deps {
		root := t.TempDir()
		dirs[i] = filepath.Join(root, "daemon")
		t.Setenv("HOME", root)
		t.Setenv("ISSUEOPS_ROOT", root)
		t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(root, "state"))
		t.Setenv("ISSUEOPS_WORKER_DIR", filepath.Join(root, "worker"))
		t.Setenv("ISSUEOPS_DAEMON_DIR", dirs[i])
		t.Setenv("ISSUEOPS_DAEMON_MAX_CONNECTIONS", strconv.Itoa(limits[i]))
		deps[i] = issueOpsMCPDependencies()
		clients[i] = startHistoryMCPTestSession(t, deps[i])
	}
	for _, sdk := range []bool{false, true} {
		for i := range deps {
			var raw string
			if sdk {
				result, err := clients[i].CallTool(context.Background(), &mcp.CallToolParams{Name: "daemon_status", Arguments: map[string]any{}})
				if err != nil || result.IsError {
					t.Fatalf("daemon SDK %d: %+v %v", i, result, err)
				}
				raw = result.Content[0].(*mcp.TextContent).Text
			} else {
				result, err := callSDKTool(t, json.RawMessage(`{"name":"daemon_status","arguments":{}}`), deps[i])
				if err != nil {
					t.Fatal(err)
				}
				raw = result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
			}
			var got contract.Status
			if err := json.Unmarshal([]byte(raw), &got); err != nil {
				t.Fatal(err)
			}
			if got.Paths.Dir != dirs[i] || got.MaxConnections != limits[i] || got.Code != contract.StatusStopped {
				t.Fatalf("daemon context leaked: instance=%d SDK=%v got=%+v", i, sdk, got)
			}
			if _, err := os.Stat(dirs[i]); !os.IsNotExist(err) {
				t.Fatalf("read-only daemon query created %s: %v", dirs[i], err)
			}
		}
	}
}
