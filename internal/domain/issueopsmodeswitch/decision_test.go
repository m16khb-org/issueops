package issueopsmodeswitch

import (
	"reflect"
	"testing"
)

func TestNormalizeModeRequiresExplicitTarget(t *testing.T) {
	for _, tt := range []struct {
		input, want string
		valid       bool
	}{
		{input: " direct ", want: "direct", valid: true},
		{input: "ORCA", want: "orca", valid: true},
		{input: "auto"},
		{input: ""},
	} {
		got, err := NormalizeMode(tt.input)
		if tt.valid && (err != nil || got != tt.want) {
			t.Fatalf("input=%q got=%q err=%v", tt.input, got, err)
		}
		if !tt.valid && err == nil {
			t.Fatalf("input=%q should be rejected", tt.input)
		}
	}
}

func TestMissingGatesKeepsAllSwitchDenialsInOrder(t *testing.T) {
	facts := Facts{
		CurrentMode: "orca", RequestedMode: "orca", WriterPresent: true, PendingIntent: true,
		WorktreePresent: true, WorktreeClean: false, NoUnpushedCommits: false, OrcaRemoteBranchExists: true,
	}
	want := []string{
		"mode_actually_changes", "lease_holds_no_writer", "pending_intent_absent",
		"worktree_clean", "worktree_commits_pushed", "orca_branch_name_free",
	}
	if got := MissingGates(facts); !reflect.DeepEqual(got, want) {
		t.Fatalf("missing=%q want=%q", got, want)
	}
	facts.CurrentMode = "direct"
	facts.WriterPresent = false
	facts.PendingIntent = false
	facts.WorktreePresent = false
	facts.OrcaRemoteBranchExists = false
	if got := MissingGates(facts); len(got) != 0 {
		t.Fatalf("absent worktree has no cleanliness or commit gate: %q", got)
	}
}
