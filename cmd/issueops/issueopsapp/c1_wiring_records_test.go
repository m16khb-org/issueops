package issueopsapp

import (
	"os"
	"path/filepath"
	"testing"

	agentmodelcontract "issueops/internal/contract/agentmodel"
	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
)

func TestReconcileContractRecordRejectsRecordsTheLeaseContractWouldNotPersist(t *testing.T) {
	valid := issueopscontract.IssueOpsRecord{SchemaVersion: leasecontract.SchemaVersion, ID: "io-1", Repo: "/r", Branch: "b", Phase: "plan"}
	got, err := reconcileContractRecord(valid)
	if err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
	if !got.OK || got.ID != "io-1" || got.Repo != "/r" || got.Branch != "b" {
		t.Fatalf("decoded record = %+v", got)
	}

	wrongSchema := valid
	wrongSchema.SchemaVersion = leasecontract.SchemaVersion + 1
	if _, err := reconcileContractRecord(wrongSchema); err == nil {
		t.Fatal("record with unsupported schema version must be rejected")
	}

	badMode := valid
	badMode.Execution = &issueopscontract.Execution{Mode: "bogus"}
	if _, err := reconcileContractRecord(badMode); err == nil {
		t.Fatal("record with invalid execution mode must be rejected as invalid state")
	}
}

func TestResumeCoreRecordRoundTripsLeaseRecordIntoCoreRecord(t *testing.T) {
	record := leasecontract.Record{
		OK: true, SchemaVersion: leasecontract.SchemaVersion, ID: "io-2", Repo: "/r", Branch: "feat",
		Phase: "implement", WorktreePath: "/r.worktrees/feat", IssueURL: "https://example.invalid/i/1",
	}
	got, err := resumeCoreRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "io-2" || got.Repo != "/r" || got.Branch != "feat" || got.Phase != "implement" ||
		got.WorktreePath != "/r.worktrees/feat" || got.IssueURL != "https://example.invalid/i/1" || got.Execution != nil {
		t.Fatalf("core record = %+v", got)
	}
}

func TestIssueOpsReseedExecutionRequiresExecutionAndCarriesIt(t *testing.T) {
	if _, err := issueOpsReseedExecution(leasecontract.Record{ID: "io"}); err == nil {
		t.Fatal("record without execution must be rejected")
	}
	record := leasecontract.Record{Execution: &leasecontract.Execution{
		Mode:      "orca",
		Workspace: leasecontract.Workspace{Root: "/wt", SourceRoot: "/src", Branch: "feat"},
	}}
	got, err := issueOpsReseedExecution(record)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "orca" || got.Workspace.Root != "/wt" || got.Workspace.SourceRoot != "/src" || got.Workspace.Branch != "feat" {
		t.Fatalf("execution = %+v", got)
	}
}

func TestConvertReconcileReceiptReportsUnencodableAndMismatchedSources(t *testing.T) {
	if _, err := convertReconcileReceipt[leasecontract.ReconcileStageReceipt](make(chan int)); err == nil {
		t.Fatal("unencodable source must fail")
	}
	if _, err := convertReconcileReceipt[leasecontract.ReconcileStageReceipt](map[string]any{"run_id": []int{1}}); err == nil {
		t.Fatal("source whose field types do not match the target must fail")
	}
	got, err := convertReconcileReceipt[map[string]int](map[string]int{"a": 1})
	if err != nil || got["a"] != 1 {
		t.Fatalf("matching shapes must convert: %v %v", got, err)
	}
}

func TestIssueOpsOrcaIntentRequestRejectsUndecodableIntent(t *testing.T) {
	if _, err := issueOpsOrcaIntentRequest(leasecontract.Record{}, "op-1", []byte("not json")); err == nil {
		t.Fatal("undecodable sealed intent must be rejected")
	}
}

func TestMemoizedReviewModelLoadsSettingsOncePerRepository(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, "issueops", "agent-models.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	repoA, repoB := t.TempDir(), t.TempDir()
	resolve := memoizedReviewModel()

	if _, _, err := resolve("claude", agentmodelcontract.RoleDiffReview, "", repoA); err == nil {
		t.Fatal("malformed settings must fail the resolution")
	}
	if err := os.Remove(settingsPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolve("claude", agentmodelcontract.RoleDiffReview, "", repoA); err == nil {
		t.Fatal("settings for a repository must be loaded once per next call, so the first load result is reused")
	}
	model, _, err := resolve("claude", agentmodelcontract.RoleDiffReview, "", repoB)
	if err != nil || model == "" {
		t.Fatalf("a different repository must load its own settings: model=%q err=%v", model, err)
	}
}
