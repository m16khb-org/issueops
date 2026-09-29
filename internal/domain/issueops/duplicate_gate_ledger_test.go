package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestApplyDuplicateGateLedgerAddsOneFailClosedKey(t *testing.T) {
	ready := model.IssueOpsReadiness{Ready: true, Missing: []string{"prior"}}
	entries := []GateLedgerFile{{Name: "issue-21-old.md"}, {Name: "21-extra.md"}}
	got := ApplyDuplicateGateLedger(ready, "21", true, entries)
	if got.Ready || !reflect.DeepEqual(got.Missing, []string{"duplicate_issue_artifact:21", "prior"}) {
		t.Fatalf("duplicate readiness=%+v", got)
	}
	if unchanged := ApplyDuplicateGateLedger(ready, "21", false, entries); !reflect.DeepEqual(unchanged, ready) {
		t.Fatalf("without canonical ledger=%+v", unchanged)
	}
}
