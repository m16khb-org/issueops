package orca

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"issueops/internal/port"
)

func TestOwnerAgentCommandRejectsUnlaunchableProfiles(t *testing.T) {
	for _, tt := range []struct {
		name, agent, model, effort string
	}{
		{"blank model", "claude", "  ", ""},
		{"NUL in model", "codex", "gpt\x00", ""},
		{"NUL in effort", "omp", "m", "high\x00"},
		{"unknown host", "gemini", "m", ""},
	} {
		if command, ok := ownerAgentCommand(tt.agent, tt.model, tt.effort, false, nil); ok {
			t.Errorf("%s: launch command %q was rendered", tt.name, command)
		}
	}
}

func TestOwnerAgentCommandQuotesAndPlacesHostFlags(t *testing.T) {
	for _, tt := range []struct {
		agent, model, effort string
		bypass               bool
		want                 string
	}{
		{"codex", "it's", "high", true, `codex --model 'it'\''s' -c model_reasoning_effort='high' --dangerously-bypass-approvals-and-sandbox --dangerously-bypass-hook-trust`},
		{"codex", "gpt", "", false, `codex --model 'gpt' --dangerously-bypass-approvals-and-sandbox`},
		{" Claude ", "opus", "max", false, `claude --model 'opus' --effort 'max' --dangerously-skip-permissions`},
		{"omo", "vendor/model", "max", false, `omo --model 'vendor/model:max'`},
		{"omo", "vendor/model", "", false, `omo --model 'vendor/model'`},
		{"omp", "m", "low", false, `omp --model 'm' --thinking 'low' --auto-approve`},
		{"omp", "m", "", false, `omp --model 'm' --auto-approve`},
	} {
		got, ok := ownerAgentCommand(tt.agent, tt.model, tt.effort, tt.bypass, nil)
		if !ok || got != tt.want {
			t.Errorf("ownerAgentCommand(%q, %q, %q, %t) = %q, %t; want %q", tt.agent, tt.model, tt.effort, tt.bypass, got, ok, tt.want)
		}
	}
}

func TestHasJSONValueTreatsEmptyStringsAndNullAsAbsent(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want bool
	}{
		{``, false},
		{`null`, false},
		{`""`, false},
		{`"  "`, false},
		{`"done"`, true},
		{`{"summary":"x"}`, true},
		{`0`, true},
		{`[]`, true},
	} {
		if got := hasJSONValue(json.RawMessage(tt.raw)); got != tt.want {
			t.Errorf("hasJSONValue(%s) = %t, want %t", tt.raw, got, tt.want)
		}
	}
}

func TestRequireCompleteListNeedsExplicitCompletenessMetadata(t *testing.T) {
	zero, two := 0, 2
	for _, tt := range []struct {
		name      string
		length    int
		total     *int
		truncated bool
		wantErr   string
	}{
		{"complete", 2, &two, false, ""},
		{"complete and empty", 0, &zero, false, ""},
		{"missing total", 2, nil, false, "completeness metadata is missing"},
		{"truncated", 2, &two, true, "truncated=true"},
		{"fewer rows than total", 1, &two, false, "totalCount=2 returned=1"},
	} {
		err := requireCompleteList("terminal", tt.length, tt.total, tt.truncated)
		if tt.wantErr == "" {
			if err != nil {
				t.Errorf("%s: unexpected error %v", tt.name, err)
			}
			continue
		}
		if orcaErr := requireOrcaError(t, err, "incomplete_list"); !strings.Contains(orcaErr.Detail, tt.wantErr) || !orcaErr.Invoked {
			t.Errorf("%s: error = %#v", tt.name, orcaErr)
		}
	}
}

func TestStableVisualTabTitlesAcceptsALayoutExactlyAtTheLimit(t *testing.T) {
	var layout visualLayoutPayload
	for range port.OrcaMaxBaselineIDs {
		layout.Root.Tabs = append(layout.Root.Tabs, struct {
			TabID        string `json:"tabId"`
			Title        string `json:"title"`
			ActiveLeafID string `json:"activeLeafId"`
		}{})
	}
	if _, err := stableVisualTabTitles([]visualLayoutPayload{layout}); err != nil {
		t.Fatalf("a layout exactly at the limit must be accepted: %v", err)
	}
}

func TestListWorktreesRequiresCompletenessMetadata(t *testing.T) {
	runner := newFakeRunner(t)
	runner.responses["orca worktree list --repo path:/repo --limit 512 --json"] = okEnvelope(`{"worktrees":[]}`)
	_, err := NewClient(runner).ListWorktrees(context.Background(), "/repo")
	if orcaErr := requireOrcaError(t, err, "incomplete_list"); !strings.Contains(orcaErr.Detail, "completeness metadata is missing") {
		t.Fatalf("detail = %q", orcaErr.Detail)
	}
}
