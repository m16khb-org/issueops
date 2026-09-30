package issueops

import (
	"reflect"
	"testing"
)

func TestDuplicateGateLedgerMissingRequiresOwnCanonicalAndLegacy(t *testing.T) {
	entries := []GateLedgerFile{
		{Name: "issue-21.md"}, {Name: "21-extra.md", Directory: true}, {Name: "248-other.md"}, {Name: "notes.txt"},
	}
	if got := DuplicateGateLedgerMissing("21", true, entries); !reflect.DeepEqual(got, []string{"duplicate_issue_artifact:21"}) {
		t.Fatalf("own duplicate=%v", got)
	}
	if got := DuplicateGateLedgerMissing("21", false, entries); len(got) != 0 {
		t.Fatalf("no canonical ledger=%v", got)
	}
	if got := DuplicateGateLedgerMissing("", true, entries); len(got) != 0 {
		t.Fatalf("unknown issue=%v", got)
	}
	if got := DuplicateGateLedgerMissing("21", true, entries[1:]); len(got) != 0 {
		t.Fatalf("other issue or directory=%v", got)
	}
}
