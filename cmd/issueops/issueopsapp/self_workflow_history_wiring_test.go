package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/selfworkflow/historycompare"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	statestore "issueops/internal/adapter/outbound/state"
	augmentapp "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
)

// Sharing the last composed state root or bypassing retention validation must
// fail this test: both clients use identical keys in different real stores.
func TestSelfWorkflowHistoryInstancesKeepCLIAndMCPStateSeparate(t *testing.T) {
	dirs := []string{t.TempDir(), t.TempDir()}
	services := make([]augmentapp.HistoryService, 2)
	sessions := make([]*mcp.ClientSession, 2)
	for i, dir := range dirs {
		for j, key := range []string{"self-verify-old", "self-verify-new"} {
			snapshot := contract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", OK: true, GeneratedAt: fmt.Sprintf("2026-01-0%dT00:00:00Z", j+1), IssueOpsRoot: fmt.Sprintf("repo-%d", i), ElapsedMS: int64(100*(i+1) + j), Summary: contract.SelfAugmentSummary{TotalRuns: 1, TotalSteps: 1, PassedSteps: 1}}
			b, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := statestore.WriteStateRecord(context.Background(), dir, key, stateRecordForHistoryTest(key, string(b))); err != nil {
				t.Fatal(err)
			}
		}
		services[i] = newSelfWorkflowHistory(dir)
		sessions[i] = startHistoryMCPTestSession(t, mcpcli.MCPDependencies{Catalog: mcpcatalog.Build(), SelfHistory: services[i]})
	}
	var wg sync.WaitGroup
	for i, service := range services {
		wg.Add(1)
		go func(i int, service augmentapp.HistoryService) {
			defer wg.Done()
			for n := 0; n < 10; n++ {
				var cli contract.SelfAugmentHistoryResult
				err := historycompare.RunSelfVerifyHistory([]string{"--json"}, historycompare.CLIDeps{History: func(prefix string, limit int, retention contract.SelfAugmentHistoryRetentionOptions) (contract.SelfAugmentHistoryResult, error) {
					return service.History(context.Background(), prefix, limit, retention)
				}, Compare: service.Compare, PrintJSON: func(v any) error { cli = v.(contract.SelfAugmentHistoryResult); return nil }})
				if err != nil || cli.StateDir != dirs[i] || len(cli.Entries) != 2 || cli.Entries[0].ElapsedMS != int64(100*(i+1)+1) {
					t.Errorf("CLI instance %d: result=%+v err=%v", i, cli, err)
					return
				}
				raw := json.RawMessage(`{"name":"self_verify_compare","arguments":{"baseline_key":"self-verify-old","candidate_key":"self-verify-new"}}`)
				result, rpcErr := callSDKTool(t, raw, mcpcli.MCPDependencies{Catalog: mcpcatalog.Build(), SelfHistory: service})
				if rpcErr != nil {
					t.Errorf("MCP direct instance %d: %v", i, rpcErr)
					return
				}
				compare, err := historyComparePayload(result)
				if err != nil || compare.StateDir != dirs[i] || compare.ElapsedDeltaMS != 1 {
					t.Errorf("MCP direct instance %d: %+v %v", i, compare, err)
					return
				}
				sdk, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify_compare", Arguments: map[string]any{"baseline_key": "self-verify-old", "candidate_key": "self-verify-new"}})
				if err != nil {
					t.Errorf("MCP SDK instance %d: %v", i, err)
					return
				}
				compare, err = historyComparePayload(sdk)
				if err != nil || compare.StateDir != dirs[i] || compare.ElapsedDeltaMS != 1 {
					t.Errorf("MCP SDK instance %d: %+v %v", i, compare, err)
					return
				}
			}
		}(i, service)
	}
	wg.Wait()
	// The domain refuses confirmation without a prune request, before any writes.
	for i, service := range services {
		err := historycompare.RunSelfVerifyHistory([]string{"--confirm", "--json"}, historycompare.CLIDeps{History: func(prefix string, limit int, retention contract.SelfAugmentHistoryRetentionOptions) (contract.SelfAugmentHistoryResult, error) {
			return service.History(context.Background(), prefix, limit, retention)
		}, Compare: service.Compare, PrintJSON: func(any) error { return nil }})
		if err == nil || !strings.Contains(err.Error(), "requires --prune-retention") {
			t.Fatalf("domain refusal: %v", err)
		}
		result, err := service.History(context.Background(), "self-verify", 0, contract.SelfAugmentHistoryRetentionOptions{})
		if err != nil || result.TotalMatches != 2 {
			t.Fatalf("refusal changed store %d: %+v %v", i, result, err)
		}
	}
	// Confirmed pruning only changes the selected instance.
	if _, err := services[0].History(context.Background(), "self-verify", 0, contract.SelfAugmentHistoryRetentionOptions{Limit: 1, PruneRequested: true, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	for i, want := range []int{1, 2} {
		result, err := services[i].History(context.Background(), "self-verify", 0, contract.SelfAugmentHistoryRetentionOptions{})
		if err != nil || result.TotalMatches != want {
			t.Fatalf("retention isolation instance %d: %+v %v", i, result, err)
		}
	}
}

func stateRecordForHistoryTest(key, content string) statecontract.RecordEnvelope {
	return statecontract.RecordEnvelope{SchemaVersion: 1, Key: key, Content: content, Bytes: len(content), UpdatedAt: "2026-01-01T00:00:00Z"}
}

func historyComparePayload(result any) (contract.SelfAugmentCompareResult, error) {
	var envelope struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	var comparison contract.SelfAugmentCompareResult
	b, err := json.Marshal(result)
	if err != nil {
		return comparison, err
	}
	if err = json.Unmarshal(b, &envelope); err != nil {
		return comparison, err
	}
	if len(envelope.Content) != 1 {
		return comparison, fmt.Errorf("unexpected MCP content: %s", b)
	}
	err = json.Unmarshal([]byte(envelope.Content[0].Text), &comparison)
	return comparison, err
}

func startHistoryMCPTestSession(t *testing.T, deps mcpcli.MCPDependencies) *mcp.ClientSession {
	t.Helper()
	server, client := net.Pipe()
	// The session lives for the test, not a fixed wall clock: a slow call under
	// full -race load must not be cut off by an unrelated deadline.
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- mcpcli.ServeMCPStreamContextWithDependencies(ctx, server, server, io.Discard, deps) }()
	session, err := mcp.NewClient(&mcp.Implementation{Name: "history-instance-test", Version: "1"}, nil).Connect(ctx, &mcp.IOTransport{Reader: client, Writer: client}, nil)
	if err != nil {
		cancel()
		_ = client.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close(); cancel(); _ = client.Close(); <-done })
	return session
}

func TestSelfWorkflowHistoryRefusalDoesNotMaterializeState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "absent")
	service := newSelfWorkflowHistory(dir)
	deps := historycompare.CLIDeps{History: func(prefix string, limit int, retention contract.SelfAugmentHistoryRetentionOptions) (contract.SelfAugmentHistoryResult, error) {
		return service.History(context.Background(), prefix, limit, retention)
	}, Compare: service.Compare, PrintJSON: func(any) error { return nil }}
	if err := historycompare.RunSelfVerifyHistory([]string{"--confirm", "--json"}, deps); err == nil || !strings.Contains(err.Error(), "requires --prune-retention") {
		t.Fatalf("history refusal: %v", err)
	}
	if err := historycompare.RunSelfVerifyCompare([]string{"--json"}, deps); err == nil || !strings.Contains(err.Error(), "baseline-key") {
		t.Fatalf("compare refusal: %v", err)
	}
	mcpDeps := mcpcli.MCPDependencies{Catalog: mcpcatalog.Build(), SelfHistory: service}
	_, rpcErr := callSDKTool(t, json.RawMessage(`{"name":"self_verify_history","arguments":{"confirm":true}}`), mcpDeps)
	if rpcErr == nil || rpcErr.Code != -32602 || !strings.Contains(string(rpcErr.Data), "requires --prune-retention") {
		t.Fatalf("MCP domain refusal: %+v", rpcErr)
	}
	session := startHistoryMCPTestSession(t, mcpDeps)
	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify_history", Arguments: map[string]any{"confirm": true}})
	var sdkError *jsonrpc.Error
	if !errors.As(err, &sdkError) || sdkError.Code != -32602 || !strings.Contains(string(sdkError.Data), "requires --prune-retention") {
		t.Fatalf("SDK domain refusal: %v (%+v)", err, sdkError)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("refused request materialized state: %v", err)
	}
}
