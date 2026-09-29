package issueopsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"issueops/internal/adapter/outbound/sqlstore"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/statecli"
	statecontract "issueops/internal/contract/state"
)

func TestPublicStateInstancesKeepToolsAndResourcesSeparate(t *testing.T) {
	var deps [2]mcpcli.MCPDependencies
	var sessions [2]*mcp.ClientSession
	var dirs [2]string
	var cli [2]statecli.Dependencies
	for i := range deps {
		dirs[i] = t.TempDir()
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(fmt.Sprintf("agents-%d", i)), 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ISSUEOPS_STATE_DIR", dirs[i])
		t.Setenv("ISSUEOPS_ROOT", root)
		cli[i] = stateDependencies()
		deps[i] = issueOpsMCPDependencies()
		sessions[i] = startHistoryMCPTestSession(t, deps[i])
	}
	// Changing the process environment after composition must not redirect either server.
	t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(t.TempDir(), "unused"))
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	// Sequential CLI stdout capture, then concurrent SDK/direct reads of the same stores.
	for i := range cli {
		raw := captureStdoutForContract(t, func() error {
			return statecli.Run(cli[i], []string{"write", "--key", "shared", "--value", fmt.Sprintf("cli-%d", i), "--json"})
		})
		var write statecontract.StateResult
		if err := json.Unmarshal([]byte(raw), &write); err != nil || write.StateDir != dirs[i] {
			t.Fatalf("CLI write: %+v %v", write, err)
		}
		result, rpcErr := mcpcli.HandleToolCallWithDependencies(json.RawMessage(`{"name":"state_read","arguments":{"key":"shared"}}`), deps[i])
		if rpcErr != nil {
			t.Fatal(rpcErr)
		}
		text := result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
		var read statecontract.StateResult
		if err := json.Unmarshal([]byte(text), &read); err != nil || !reflect.DeepEqual(write, read) {
			t.Fatalf("CLI/MCP: write=%+v read=%+v err=%v", write, read, err)
		}
	}
	var wg sync.WaitGroup
	for i := range deps {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for n := 0; n < 5; n++ {
				want := fmt.Sprintf("owner-%d-%d", i, n)
				_, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: "state_write", Arguments: map[string]any{"key": "shared", "content": want}})
				if err != nil {
					t.Error(err)
					return
				}
				result, rpcErr := mcpcli.HandleToolCallWithDependencies(json.RawMessage(`{"name":"state_read","arguments":{"key":"shared"}}`), deps[i])
				if rpcErr != nil {
					t.Error(rpcErr)
					return
				}
				text := result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
				var read statecontract.StateResult
				if err := json.Unmarshal([]byte(text), &read); err != nil || read.StateDir != dirs[i] || read.Record.Content != want {
					t.Errorf("instance %d: %+v, %v", i, read, err)
					return
				}
				for _, uri := range []string{"issueops://state", "issueops://agents"} {
					resource, err := sessions[i].ReadResource(context.Background(), &mcp.ReadResourceParams{URI: uri})
					if err != nil {
						t.Error(err)
						return
					}
					text := resource.Contents[0].Text
					raw, _ := json.Marshal(map[string]string{"uri": uri})
					direct, rpcErr := mcpcli.HandleResourceRead(raw, deps[i].Resources)
					if rpcErr != nil {
						t.Error(rpcErr)
						return
					}
					directText := direct.(map[string]any)["contents"].([]map[string]any)[0]["text"].(string)
					if directText != text {
						t.Errorf("direct/SDK resource mismatch: %s / %s", directText, text)
						return
					}

					if uri == "issueops://agents" {
						if text != fmt.Sprintf("agents-%d", i) {
							t.Errorf("resource owner %d: %s", i, text)
						}
					} else {
						var list statecontract.StateListResult
						if err := json.Unmarshal([]byte(text), &list); err != nil || list.StateDir != dirs[i] || len(list.Keys) != 1 || list.Keys[0] != "shared" {
							t.Errorf("resource owner %d: %+v, %v", i, list, err)
						}
					}
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestPublicStateRefusalAndMissingReadsDoNotCreateStores(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	t.Setenv("ISSUEOPS_WORKER_DIR", filepath.Join(t.TempDir(), "worker"))
	cli := stateDependencies()
	deps := issueOpsMCPDependencies()
	t.Setenv("ISSUEOPS_STATE_DIR", filepath.Join(t.TempDir(), "other"))
	if err := statecli.Run(cli, []string{"write", "--key", "../invalid", "--value", "x"}); err == nil {
		t.Fatal("invalid key accepted")
	}
	for _, args := range []string{
		`{"name":"state_write","arguments":{"key":"../invalid","content":"x"}}`,
		`{"name":"state_read","arguments":{"key":"missing"}}`,
		`{"name":"state_prune","arguments":{"max_age":"0s","confirm":true}}`,
	} {
		if _, err := mcpcli.HandleToolCallWithDependencies(json.RawMessage(args), deps); err == nil {
			t.Fatalf("request accepted: %s", args)
		}
	}
	doctor, err := cli.Doctor()
	if err != nil || !doctor.Healthy || doctor.StateDir != dir {
		t.Fatalf("doctor: %+v %v", doctor, err)
	}
	maintain, err := cli.Maintain()
	if err != nil || !maintain.OK || len(maintain.Roots) != 0 || !strings.Contains(strings.Join(maintain.Skipped, "\n"), dir) {
		t.Fatalf("maintain: %+v %v", maintain, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("missing state materialized: %v", err)
	}
}

func TestPublicStateUnknownSchemaIsRefusedWithoutRewriting(t *testing.T) {
	for _, schema := range []string{"", `"schema_version":0,`, `"schema_version":2,`} {
		t.Run(schema, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("ISSUEOPS_STATE_DIR", dir)
			raw := []byte(`{` + schema + `"key":"future","content":"abc","updated_at":"2000-01-01T00:00:00Z","bytes":3}`)
			db, err := sqlstore.Open(dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Put("state", "future", raw); err != nil {
				t.Fatal(err)
			}
			cli := stateDependencies()
			deps := issueOpsMCPDependencies()
			if err := statecli.Run(cli, []string{"read", "--key", "future"}); err == nil || err.Error() != "invalid state" {
				t.Fatalf("CLI schema refusal: %v", err)
			}
			_, rpcErr := mcpcli.HandleToolCallWithDependencies(json.RawMessage(`{"name":"state_read","arguments":{"key":"future"}}`), deps)
			if rpcErr == nil || rpcErr.Code != -32602 || string(rpcErr.Data) != `"invalid state"` {
				t.Fatalf("MCP schema refusal: %+v", rpcErr)
			}
			session := startHistoryMCPTestSession(t, deps)
			_, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "state_read", Arguments: map[string]any{"key": "future"}})
			var protocolErr *jsonrpc.Error
			if !errors.As(err, &protocolErr) || protocolErr.Code != -32602 || string(protocolErr.Data) != `"invalid state"` {
				t.Fatalf("SDK schema refusal: %v (%+v)", err, protocolErr)
			}
			doctor, err := cli.Doctor()
			if err != nil || doctor.Healthy || len(doctor.Issues) != 1 || doctor.Issues[0].Code != "invalid_state" {
				t.Fatalf("doctor: %+v %v", doctor, err)
			}
			after, ok, err := sqlstore.GetExisting(dir, "state", "future")
			if err != nil || !ok || !bytes.Equal(raw, after) {
				t.Fatalf("schema refusal rewrote record: %s %v", after, err)
			}
		})
	}
}

func TestPublicStatePrunePreviewAndMaintenanceUseCapturedRoots(t *testing.T) {
	dir := t.TempDir()
	worker := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	t.Setenv("ISSUEOPS_WORKER_DIR", worker)
	deps := issueOpsMCPDependencies()
	cli := stateDependencies()
	old := []byte(`{"schema_version":1,"key":"old","content":"abc","updated_at":"2000-01-01T00:00:00Z","bytes":3}`)
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put("state", "old", old); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{worker, filepath.Join(dir, "projects", "one"), filepath.Join(dir, "loop")} {
		store, err := sqlstore.Open(root)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Put("fixture", "marker", []byte("kept")); err != nil {
			t.Fatal(err)
		}
	}
	other := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", other)
	t.Setenv("ISSUEOPS_WORKER_DIR", other)
	session := startHistoryMCPTestSession(t, deps)
	preview, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "state_prune", Arguments: map[string]any{"max_age": "1h"}})
	if err != nil {
		t.Fatal(err)
	}
	var prune statecontract.StatePruneResult
	if err := json.Unmarshal([]byte(preview.Content[0].(*mcp.TextContent).Text), &prune); err != nil || !prune.DryRun || !reflect.DeepEqual(prune.DeletedKeys, []string{"old"}) {
		t.Fatalf("preview: %+v %v", prune, err)
	}
	after, ok, err := sqlstore.GetExisting(dir, "state", "old")
	if err != nil || !ok || !bytes.Equal(old, after) {
		t.Fatalf("preview rewrote record: %s %v", after, err)
	}
	maintain, err := cli.Maintain()
	if err != nil || !maintain.OK || len(maintain.Roots) != 4 {
		t.Fatalf("maintenance: %+v %v", maintain, err)
	}
	for _, root := range maintain.Roots {
		if root.Dir != worker && !strings.HasPrefix(root.Dir, dir) {
			t.Fatalf("maintenance redirected: %+v", root)
		}
	}
	if _, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "state_prune", Arguments: map[string]any{"max_age": "1h", "confirm": true}}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := sqlstore.GetExisting(dir, "state", "old"); err != nil || ok {
		t.Fatalf("confirmed prune did not delete: %v %v", ok, err)
	}
}
