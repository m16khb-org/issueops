package mcpcli

import (
	"context"
	"encoding/json"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	"strings"
	"testing"
)

func TestHandleSelfLoopMCPToolCallCoversLocalPayloads(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	tests := []struct {
		name     string
		call     MCPToolCall
		wantText string
	}{
		{
			name: "self augment plan",
			call: MCPToolCall{Name: "self_augment", Arguments: map[string]any{
				"cycles": 2,
			}},
			wantText: `"self_augmentation"`,
		},
		{
			name: "self augment plan save state",
			call: MCPToolCall{Name: "self_augment", Arguments: map[string]any{
				"save_state": true,
				"state_key":  "mcp-self-augment-plan",
			}},
			wantText: `"state_checkpoint"`,
		},
		{
			name: "self augment lesson",
			call: MCPToolCall{Name: "self_augment_lesson", Arguments: map[string]any{
				"candidate_id": "candidate-refill-curriculum",
				"lesson":       "Keep MCP self dispatch local in tests.",
				"next_action":  "Pin the next safe branch.",
			}},
			wantText: `"self_augmentation_lesson"`,
		},
		{
			name:     "self verify candidates",
			call:     MCPToolCall{Name: "self_verify_candidates", Arguments: map[string]any{}},
			wantText: `"self_verification_candidate_export"`,
		},
		{
			name:     "self verify candidates save state",
			call:     MCPToolCall{Name: "self_verify_candidates", Arguments: map[string]any{"save_state": true, "state_key": "mcp-self-candidates"}},
			wantText: `"state_checkpoint"`,
		},
		{
			name:     "self verify history",
			call:     MCPToolCall{Name: "self_verify_history", Arguments: map[string]any{"prefix": "mcp-self", "limit": 5}},
			wantText: `"entries"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := testHandleSelfLoopMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err != nil {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
			if text := mcpSelfPayloadText(t, outcome.Payload); !strings.Contains(text, tc.wantText) {
				t.Fatalf("payload text = %s, want %q", text, tc.wantText)
			}
		})
	}
}

func TestHandleSelfLoopMCPToolCallCoversBoundaryErrorsAndUnknownTool(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	tests := []struct {
		name    string
		call    MCPToolCall
		wantMsg string
	}{
		{
			name:    "self augment lesson missing lesson",
			call:    MCPToolCall{Name: "self_augment_lesson", Arguments: map[string]any{"candidate_id": "candidate-refill-curriculum", "next_action": "y"}},
			wantMsg: "Self-augmentation lesson save failed",
		},
		{
			name:    "self verify compare missing baseline",
			call:    MCPToolCall{Name: "self_verify_compare", Arguments: map[string]any{"candidate_key": "candidate"}},
			wantMsg: "Self-verify compare failed",
		},
		{
			name:    "self verify promote missing source",
			call:    MCPToolCall{Name: "self_verify_promote", Arguments: map[string]any{"baseline_key": "baseline", "confirm": true}},
			wantMsg: "Self-verify promote failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome := testHandleSelfLoopMCPToolCall(tc.call)
			if !outcome.Handled || outcome.Err == nil || outcome.Err.Message != tc.wantMsg {
				t.Fatalf("unexpected MCP outcome: %#v", outcome)
			}
		})
	}

	unknown := testHandleSelfLoopMCPToolCall(MCPToolCall{Name: "not_self_loop", Arguments: map[string]any{}})
	if unknown.Handled {
		t.Fatalf("unknown self-loop tool should pass through: %#v", unknown)
	}
}

func mcpSelfPayloadText(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSelfPlanAndCandidatesReturnFailedCheckpointOnSaveError(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	for _, tool := range []string{"self_augment", "self_verify_candidates"} {
		t.Run(tool, func(t *testing.T) {
			outcome := testHandleSelfLoopMCPToolCall(MCPToolCall{Name: tool, Arguments: map[string]any{"save_state": true, "state_key": "!invalid-key"}})
			if !outcome.Handled || outcome.Err == nil || outcome.Err.Code != -32000 {
				t.Fatalf("outcome=%+v", outcome)
			}
			var data struct {
				StateCheckpoint struct {
					OK    bool   `json:"ok"`
					Key   string `json:"key"`
					Error string `json:"error"`
				} `json:"state_checkpoint"`
			}
			if err := json.Unmarshal(outcome.Err.Data, &data); err != nil {
				t.Fatal(err)
			}
			if data.StateCheckpoint.OK || data.StateCheckpoint.Key != "!invalid-key" || data.StateCheckpoint.Error == "" {
				t.Fatalf("lost failed checkpoint: %s", outcome.Err.Data)
			}
		})
	}
}

func TestSelfVerifyMCPForwardsBaseRefAndRejectsInvalidScope(t *testing.T) {
	var got string
	deps := MCPDependencies{SelfVerify: func(request verifyapp.LoopRequest) (augmentcontract.SelfAugmentResult, error) {
		got = request.BaseRef
		return augmentcontract.SelfAugmentResult{OK: true}, nil
	}}
	outcome := handleSelfLoopMCPToolCall(context.Background(), MCPToolCall{Name: "self_verify", Arguments: map[string]any{"base_ref": "HEAD~1"}}, deps)
	if outcome.Err != nil || got != "HEAD~1" {
		t.Fatalf("base_ref=%q outcome=%+v", got, outcome)
	}
	for _, value := range []any{"", 7, nil} {
		outcome := handleSelfLoopMCPToolCall(context.Background(), MCPToolCall{Name: "self_verify", Arguments: map[string]any{"base_ref": value}}, MCPDependencies{})
		if outcome.Err == nil || outcome.Err.Code != -32602 {
			t.Fatalf("invalid scope accepted: %+v", outcome)
		}
	}
}
