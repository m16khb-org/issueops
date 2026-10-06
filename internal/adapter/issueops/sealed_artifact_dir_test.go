package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

func TestSealedArtifactDirUsesOnlyRecordField(t *testing.T) {
	empty := issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/21", Execution: &issueops.Execution{}}
	if got := sealedArtifactDir(empty); got != "" {
		t.Fatalf("empty artifact_dir must not be inferred from the issue URL, got %s", got)
	}
	filled := issueops.IssueOpsRecord{Execution: &issueops.Execution{Workspace: issueops.Workspace{ArtifactDir: ".issueops/issues/21/artifact"}}}
	if got := sealedArtifactPath(filled, "/wt", "plan"); got != filepath.Join("/wt", ".issueops", "issues", "21", "artifact", "plan.md") {
		t.Fatalf("artifact_dir must drive the sealed path, got %s", got)
	}
}

func TestIssueArtifactDirForUsesLinkedIssueNumber(t *testing.T) {
	if got := issueArtifactDirFor(issueops.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/21"}); got != ".issueops/issues/21/artifact" {
		t.Fatalf("linked issue must pick the issue folder, got %q", got)
	}
	if got := issueArtifactDirFor(issueops.IssueOpsRecord{BranchPrepare: &issueops.IssueOpsBranchPrepare{IssueURL: "https://gitlab.example.com/g/p/-/work_items/7"}}); got != ".issueops/issues/7/artifact" {
		t.Fatalf("branch prepare issue URL must be a fallback, got %q", got)
	}
	if got := issueArtifactDirFor(issueops.IssueOpsRecord{}); got != "" {
		t.Fatalf("no issue number must leave artifact_dir empty, got %q", got)
	}
}

func TestMaterializeStagedArtifactsWritesIntoRecordedArtifactDir(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	root := t.TempDir()
	record.WorktreePath = root
	record.Execution = artifactRecoveryExecution(issueops.ExecutionModeOrca, issueops.LeaseStatusReleased)
	record.Execution.Workspace.SourceRoot = record.Repo
	record.Execution.Workspace.Root = root
	record.Execution.Workspace.Branch = record.Branch
	record.Execution.Workspace.BaseHead = record.BranchPrepare.BaseSHA
	record.Execution.Workspace.LinkedAt = "2026-08-27T00:00:00Z"
	record.Execution.Workspace.ArtifactDir = ".issueops/issues/480/artifact"
	if _, err := writeIssueOps(context.Background(), stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# plan\n")); err != nil {
		t.Fatal(err)
	}
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	path := filepath.Join(root, ".issueops", "issues", "480", "artifact", "plan.md")
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("plan must be sealed 0600 at the recorded dir: %v %v", err, info)
	}
	if _, ok := manifest["plan"]; !ok {
		t.Fatalf("manifest must carry plan: %+v", manifest)
	}
	// 재-materialize는 같은 내용이면 통과하고(불변 계약), 파일은 그대로다.
	if _, err := materializeStagedArtifacts(stateRoot, record); err != nil {
		t.Fatalf("idempotent re-materialize must pass: %v", err)
	}
}

func TestMaterializeStagedArtifactsRejectsMissingArtifactDir(t *testing.T) {
	stateRoot, record := executionPrepareRecord(t)
	record.Execution = artifactRecoveryExecution(issueops.ExecutionModeOrca, issueops.LeaseStatusReleased)
	record.Execution.Workspace.Root = t.TempDir()
	record.Execution.Workspace.ArtifactDir = ""
	if _, err := materializeStagedArtifacts(stateRoot, record); err == nil || !strings.Contains(err.Error(), "artifact_dir is missing") {
		t.Fatalf("missing artifact_dir must be rejected, err=%v", err)
	}
}
