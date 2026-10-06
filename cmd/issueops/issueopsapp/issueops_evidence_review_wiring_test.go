package issueopsapp

import (
	"context"
	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEvidenceReviewRuntimePersistsObservedChangeSet(t *testing.T) {
	repo := makeGitRepoForContract(t)
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("# Review fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := issueOpsStateRoot()
	runtime := newIssueOpsCLIRuntime(root)
	record, err := runtime.StartIssueOps(root, model.IssueOpsStartRequest{Repo: repo, Branch: "901-review"})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = model.IssueOpsPhaseImplement
	if _, err = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	actor := model.IssueOpsActor{}
	record, err = runtime.RecordIssueOpsImplementationReviewWithActor(root, record.ID, model.IssueOpsImplementationReviewRequest{Verdict: "pass", Findings: []string{"reviewed"}, Evidence: []string{"checked"}}, actor)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := record.ImplementationReview.ReviewedFingerprint
	if fingerprint == "" {
		t.Fatal("implementation review lost actual change fingerprint")
	}
	record, err = runtime.RecordIssueOpsProjectDocsReviewWithActor(root, record.ID, model.IssueOpsProjectDocsReviewRequest{Verdict: "updated", Docs: []string{filepath.Join(repo, "AGENTS.md")}, ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{"updated"}}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if record.ProjectDocsReview.ReviewedFingerprint != fingerprint || !reflect.DeepEqual(record.ProjectDocsReview.Docs, []string{"AGENTS.md"}) {
		t.Fatalf("docs observation=%+v", record.ProjectDocsReview)
	}
	record, err = runtime.RecordIssueOpsSchemaEvidenceWithActor(root, record.ID, model.IssueOpsSchemaEvidenceRequest{Measurements: []string{"fixture has one document"}, Sources: []string{"local fixture"}}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if record.SchemaEvidence.ReviewedFingerprint != fingerprint {
		t.Fatal("schema evidence lost fingerprint")
	}
	saved, err := core.ReadIssueOps(root, record.ID)
	if err != nil || !reflect.DeepEqual(saved, record) {
		t.Fatalf("durable evidence mismatch: %v", err)
	}
	for _, req := range []model.IssueOpsProjectDocsReviewRequest{
		{Verdict: "updated", Docs: []string{"unchanged.md"}, Evidence: []string{"claimed"}},
		{Verdict: "no-change", ReviewedDocs: []string{"README.md"}, Evidence: []string{"read"}},
		{Verdict: "no-change", ReviewedDocs: []string{"../AGENTS.md"}, Evidence: []string{"read"}},
	} {
		if _, err := runtime.RecordIssueOpsProjectDocsReviewWithActor(root, record.ID, req, actor); err == nil || !strings.Contains(err.Error(), "project docs review") {
			t.Fatalf("invalid review accepted: %+v err=%v", req, err)
		}
		latest, err := core.ReadIssueOps(root, record.ID)
		if err != nil || !reflect.DeepEqual(latest, saved) {
			t.Fatalf("rejected review changed record: %v", err)
		}
	}
}
