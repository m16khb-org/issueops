package issueopscli

import (
	"encoding/json"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

// --field keeps id/phase and the asked fields, lists unrecorded ones as
// absent, and refuses a name the record does not have.
func TestStatusProjectionKeepsRequestedFields(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{OK: true, ID: "io-1", Phase: "plan", PlanPath: "/w/plan.md", Repo: "/r"}
	got, err := statusProjection(record, []string{"plan_path,remote_artifact"})
	if err != nil {
		t.Fatal(err)
	}
	if got["id"] != "io-1" || got["repo"] != nil || len(got["absent"].([]string)) != 1 || string(got["plan_path"].(json.RawMessage)) != `"/w/plan.md"` {
		t.Fatalf("projection = %#v", got)
	}
	if _, err := statusProjection(record, []string{"plan_pth"}); err == nil || !strings.Contains(err.Error(), "did you mean: plan_path") {
		t.Fatalf("unknown field err = %v", err)
	}
}
