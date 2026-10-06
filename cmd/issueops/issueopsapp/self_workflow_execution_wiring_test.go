package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/selfworkflow/verifycmd"
	catalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/outbound/sqlstore"
	app "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSelfVerificationInstancesKeepActualFailuresAndSavedRootsSeparate(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	dirs := []string{t.TempDir(), t.TempDir()}
	var wg sync.WaitGroup
	for i := range roots {
		execute := newSelfWorkflowExecutor(roots[i])
		state := newSelfWorkflowState(dirs[i])
		session := startHistoryMCPTestSession(t, mcpcli.MCPDependencies{Catalog: catalog.Build(), SelfVerify: execute, SelfState: state})
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var cli contract.SelfAugmentResult
			err := verifycmd.Run([]string{"--seed", "42", "--llm-eval=false", "--save-state", "--state-key", "cli", "--json"}, verifycmd.Deps{Verify: execute, SaveSummary: func(result *contract.SelfAugmentResult, key string) error {
				return state.SaveSummary(context.Background(), result, key)
			}, PrintJSON: func(value any) error { cli = value.(contract.SelfAugmentResult); return nil }})
			if !errors.Is(err, app.ErrSelfVerificationGateFailed) {
				t.Errorf("instance %d CLI lost real gate error: %v", i, err)
				return
			}
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify", Arguments: map[string]any{"seed": 42, "save_state": true, "state_key": "mcp"}})
			if err != nil || result.IsError {
				t.Errorf("instance %d MCP rejected gate result: %v %+v", i, err, result)
				return
			}
			var sdk contract.SelfAugmentResult
			if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &sdk); err != nil {
				t.Error(err)
				return
			}
			for _, result := range []contract.SelfAugmentResult{cli, sdk} {
				if result.OK || result.IssueOpsRoot != roots[i] || len(result.Runs) != 1 || len(result.Runs[0].Steps) != 1 || result.Runs[0].Steps[0].OK || result.Runs[0].Steps[0].Label != "harness invariants" {
					t.Errorf("mixed root or bypassed actual failed gate: %+v", result)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	for i, dir := range dirs {
		for _, key := range []string{"cli", "mcp"} {
			raw, exists, err := sqlstore.GetExisting(dir, "state", key)
			if err != nil || !exists {
				t.Fatalf("instance %d %s not saved: %v %v", i, key, exists, err)
			}
			var record statecontract.RecordEnvelope
			if err := json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			var saved contract.SelfAugmentResult
			if err := json.Unmarshal([]byte(record.Content), &saved); err != nil {
				t.Fatal(err)
			}
			if saved.OK || saved.IssueOpsRoot != roots[i] || saved.Summary.FailedSteps != 1 {
				t.Fatalf("wrong durable failure summary: %+v", saved)
			}
		}
	}
}
