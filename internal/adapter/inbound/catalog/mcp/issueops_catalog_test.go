package mcp

import contract "issueops/internal/contract/mcp"

import (
	"reflect"
	"testing"
)

func TestIssueOpsAdvertisesOnlyExecutionActionTool(t *testing.T) {
	tools := contract.IssueOpsBasicTools()
	if len(tools) != 1 || tools[0].Name != "issueops_execution" {
		t.Fatalf("IssueOps MCP tools = %#v, want only issueops_execution", tools)
	}

	properties, ok := tools[0].InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v", tools[0].InputSchema["properties"])
	}
	action, ok := properties["action"].(map[string]any)
	if !ok {
		t.Fatalf("action schema = %#v", properties["action"])
	}
	want := []string{"prepare", "status", "claim", "release", "replace", "reconcile", "complete"}
	if got := action["enum"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("action enum = %#v, want %#v", got, want)
	}
	host, ok := properties["host"].(map[string]any)
	if !ok {
		t.Fatalf("host schema = %#v", properties["host"])
	}
	if got, want := host["enum"], []string{"codex", "claude", "omo", "omp", "opencode"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("host enum = %#v, want %#v", got, want)
	}
	claimCurrentToken, ok := properties["claim_current_token"].(map[string]any)
	if !ok || claimCurrentToken["type"] != "boolean" {
		t.Fatalf("claim_current_token schema = %#v, want boolean", properties["claim_current_token"])
	}
}

func TestIssueOpsExecutionSchemaOmitsRetiredFields(t *testing.T) {
	tools := contract.IssueOpsBasicTools()
	properties := tools[0].InputSchema["properties"].(map[string]any)
	for _, retired := range []string{"mode", "owner_host", "owner_model", "owner_effort", "direct_reason", "issue_body" + "_sha256", "context_packet" + "_sha256", "issue_" + "snapshot"} {
		if _, ok := properties[retired]; ok {
			t.Fatalf("issueops_execution schema still advertises retired field %q", retired)
		}
	}
}

func TestIssueOpsDispatchContainsNoRetiredTools(t *testing.T) {
	dispatch := DispatchMap()
	if got := dispatch["issueops_execution"]; got != contract.DispatchIssueOps {
		t.Fatalf("issueops_execution dispatch = %q", got)
	}
	for name := range dispatch {
		if name != "issueops_execution" && len(name) >= len("issueops_") && name[:len("issueops_")] == "issueops_" {
			t.Fatalf("retired IssueOps MCP tool remains advertised: %s", name)
		}
	}
}
