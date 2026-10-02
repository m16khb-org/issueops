package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/selfworkflow/promotecmd"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
)

// A shared state root, dropped destination write or bypassed promotion gate must
// fail against real saved records, through both CLI and MCP SDK entrypoints.
func TestSelfWorkflowStateInstancesSaveAndPromoteIndependently(t *testing.T) {
	dirs := []string{t.TempDir(), t.TempDir()}
	services := []mcpcli.SelfStateDependencies{newSelfWorkflowState(dirs[0]), newSelfWorkflowState(dirs[1])}
	sessions := make([]*mcp.ClientSession, 2)
	for i := range services {
		sessions[i] = startHistoryMCPTestSession(t, mcpcli.MCPDependencies{Catalog: mcpcatalog.Build(), SelfState: services[i]})
	}
	var wg sync.WaitGroup
	for i := range services {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			service := services[i]
			summary := contract.SelfAugmentResult{OK: true, IssueOpsRoot: fmt.Sprintf("repo-%d", i), Summary: contract.SelfAugmentSummary{TerminationEligible: true}}
			if err := service.SaveSummary(context.Background(), &summary, "source"); err != nil {
				t.Error(err)
				return
			}
			plan := contract.SelfAugmentPlanResult{OK: true, IssueOpsRoot: fmt.Sprintf("repo-%d", i)}
			if err := service.SavePlan(context.Background(), &plan, "plan"); err != nil {
				t.Error(err)
				return
			}
			if summary.StateCheckpoint == nil || summary.StateCheckpoint.StateDir != dirs[i] || plan.StateCheckpoint == nil || plan.StateCheckpoint.StateDir != dirs[i] {
				t.Errorf("checkpoint escaped instance %d: %+v %+v", i, summary.StateCheckpoint, plan.StateCheckpoint)
				return
			}
			var promoted contract.SelfAugmentPromoteResult
			if err := promotecmd.Run([]string{"--from-key", "source", "--baseline-key", "cli-baseline", "--confirm", "--json"}, promotecmd.Deps{Promote: func(from, to string, confirm, allowFailed bool) (contract.SelfAugmentPromoteResult, error) {
				return service.Promote(context.Background(), from, to, confirm, allowFailed)
			}, PrintJSON: func(value any) error { promoted = value.(contract.SelfAugmentPromoteResult); return nil }}); err != nil || !promoted.Promoted || promoted.StateDir != dirs[i] {
				t.Errorf("CLI promotion instance %d: %+v %v", i, promoted, err)
				return
			}
			for _, confirm := range []bool{false, true} {
				result, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify_promote", Arguments: map[string]any{"from_key": "source", "baseline_key": "sdk-baseline", "confirm": confirm}})
				if err != nil || result.IsError {
					t.Errorf("MCP promotion instance %d: %+v %v", i, result, err)
					return
				}
				var got contract.SelfAugmentPromoteResult
				if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &got); err != nil || got.Promoted != confirm || got.StateDir != dirs[i] {
					t.Errorf("MCP promotion result %+v %v", got, err)
					return
				}
				if !confirm {
					_, exists, err := sqlstore.GetExisting(dirs[i], "state", "sdk-baseline")
					if err != nil || exists {
						t.Errorf("dry run wrote baseline: exists=%v err=%v", exists, err)
						return
					}
				}
			}
		}(i)
	}
	wg.Wait()
	for i, dir := range dirs {
		for _, key := range []string{"source", "plan", "cli-baseline", "sdk-baseline"} {
			raw, exists, err := sqlstore.GetExisting(dir, "state", key)
			if err != nil || !exists {
				t.Fatalf("read %d/%s: %v exists=%v", i, key, err, exists)
			}
			var record statecontract.RecordEnvelope
			if err := json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			var snapshot struct {
				SchemaVersion int    `json:"schema_version"`
				IssueOpsRoot  string `json:"issueops_root"`
			}
			if err := json.Unmarshal([]byte(record.Content), &snapshot); err != nil {
				t.Fatal(err)
			}
			if record.SchemaVersion != 1 || snapshot.SchemaVersion != 1 || snapshot.IssueOpsRoot != fmt.Sprintf("repo-%d", i) {
				t.Fatalf("cross-instance record %d/%s: %+v", i, key, snapshot)
			}
		}
	}
}

func TestSelfWorkflowPromotionRefusesFailedAndUnknownSummaryWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	service := newSelfWorkflowState(dir)
	failed := contract.SelfAugmentResult{OK: false, Summary: contract.SelfAugmentSummary{TerminationEligible: false}}
	if err := service.SaveSummary(context.Background(), &failed, "failed"); err != nil {
		t.Fatal(err)
	}
	before, _, err := sqlstore.GetExisting(dir, "state", "failed")
	if err != nil {
		t.Fatal(err)
	}
	deps := mcpcli.MCPDependencies{Catalog: mcpcatalog.Build(), SelfState: service}
	params, _ := json.Marshal(mcpcli.MCPToolCall{Name: "self_verify_promote", Arguments: map[string]any{"from_key": "failed", "baseline_key": "forbidden", "confirm": true}})
	_, rpcErr := callSDKTool(t, params, deps)
	if rpcErr == nil || rpcErr.Code != -32602 || !strings.Contains(string(rpcErr.Data), "refusing to promote") {
		t.Fatalf("MCP gate refused incorrectly: %+v", rpcErr)
	}
	session := startHistoryMCPTestSession(t, deps)
	_, err = session.CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify_promote", Arguments: map[string]any{"from_key": "failed", "baseline_key": "forbidden", "confirm": true}})
	var sdkErr *jsonrpc.Error
	if !errors.As(err, &sdkErr) || sdkErr.Code != -32602 || !strings.Contains(string(sdkErr.Data), "refusing to promote") {
		t.Fatalf("SDK gate refused incorrectly: %v", err)
	}
	if err := promotecmd.Run([]string{"--from-key", "failed", "--baseline-key", "forbidden", "--confirm", "--json"}, promotecmd.Deps{Promote: func(from, to string, confirm, allowFailed bool) (contract.SelfAugmentPromoteResult, error) {
		return service.Promote(context.Background(), from, to, confirm, allowFailed)
	}, PrintJSON: func(any) error { return nil }}); err == nil || !strings.Contains(err.Error(), "refusing to promote") {
		t.Fatalf("CLI gate refused incorrectly: %v", err)
	}
	if _, exists, err := sqlstore.GetExisting(dir, "state", "forbidden"); err != nil || exists {
		t.Fatalf("failed source promoted: exists=%v %v", exists, err)
	}
	after, _, err := sqlstore.GetExisting(dir, "state", "failed")
	if err != nil || string(before) != string(after) {
		t.Fatalf("refusal changed source: %v", err)
	}
	if _, err := statestore.WriteStateRecord(context.Background(), dir, "future", stateRecordForHistoryTest("future", `{"schema_version":2,"kind":"self_verification_summary","ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Promote(context.Background(), "future", "forbidden", true, true); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("future schema accepted: %v", err)
	}
	if _, exists, err := sqlstore.GetExisting(dir, "state", "forbidden"); err != nil || exists {
		t.Fatalf("future schema wrote destination: exists=%v %v", exists, err)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	if _, err := newSelfWorkflowState(absent).Promote(context.Background(), "", "baseline", true, false); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("refusal materialized storage: %v", err)
	}
}
