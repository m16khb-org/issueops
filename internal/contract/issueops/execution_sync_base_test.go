package issueops

import (
	"strings"
	"testing"
)

func fullOID() string { return strings.Repeat("a", 40) }

func TestBaseSyncRequiredErrorCarriesReseedFreeNextCommand(t *testing.T) {
	err := NewBaseSyncRequiredError("io-9'x", 7)

	if got := err.Error(); !strings.Contains(got, "io-9'x") || !strings.Contains(got, "completion generation 7") {
		t.Fatalf("error message mismatch: %q", got)
	}
	if !strings.Contains(err.NextCommand, "'io-9'\\''x'") || !strings.Contains(err.NextCommand, "--completion-generation 7") {
		t.Fatalf("next command quoting wrong: %q", err.NextCommand)
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "post_completion_sync_base_required" || fields["completion_generation"] != uint64(7) {
		t.Fatalf("fields mismatch: %+v", fields)
	}
	if fields["next_command"] != err.NextCommand {
		t.Fatal("next_command field must match error field")
	}
}
