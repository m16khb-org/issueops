package issueopspreparation

import (
	"encoding/json"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestApplyOrcaBeginSealsCurrentIssueAndRejectsDrift(t *testing.T) {
	record := leasecontract.Record{ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21", BranchPrepare: json.RawMessage(`{"provider":"github","issue_url":"https://github.com/acme/repo/issues/21","link_verified":true}`)}
	begin := OrcaBegin{
		Workspace:   preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/source", Root: "/worktree", Branch: "branch"},
		Command:     preparationcontract.Command{OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high"},
		Probe:       preparationcontract.ProbeRequest{Repo: "/source", Host: "codex", Model: "model", Effort: "high", Provider: "github", Issue: 21},
		Owner:       preparationcontract.OwnerEvidence{Provider: "github", Issue: 21, BodySHA256: strings.Repeat("a", 64)},
		OperationID: "0123456789abcdef0123456789abcdef", StartedAt: "2026-08-02T00:00:00Z",
	}
	after, intent, err := ApplyOrcaBegin(record, begin, "artifact-dir")
	if err != nil {
		t.Fatal(err)
	}
	if record.Execution != nil || after.Execution == nil || after.Execution.Pending == nil || after.Execution.Pending.Marker != intent.Marker || after.Execution.Workspace.ArtifactDir != "artifact-dir" || intent.Probe.Issue != 21 {
		t.Fatalf("record=%+v after=%+v intent=%+v", record, after, intent)
	}
	begin.Owner.Issue = 22
	if _, _, err := ApplyOrcaBegin(record, begin, "artifact-dir"); err == nil || !strings.Contains(err.Error(), "owner issue identity changed") {
		t.Fatalf("issue drift error=%v", err)
	}
}
