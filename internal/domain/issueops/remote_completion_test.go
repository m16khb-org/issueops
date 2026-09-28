package issueops

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestRemoteCompletionReceiptsPreserveFirstCloseAndInput(t *testing.T) {
	record := model.IssueOpsRecord{RemoteCompletion: &model.IssueOpsRemoteCompletion{IssueClosedAt: "first", ReflectedAt: "before"}}
	closed := MarkRemoteIssueClosed(record, "second")
	reflected := MarkRemoteCompletionReflected(record, "after")
	if closed.RemoteCompletion.IssueClosedAt != "first" || closed.UpdatedAt != "second" {
		t.Fatalf("close=%+v", closed.RemoteCompletion)
	}
	if reflected.RemoteCompletion.ReflectedAt != "after" || reflected.RemoteCompletion.IssueClosedAt != "first" || record.RemoteCompletion.ReflectedAt != "before" {
		t.Fatal("reflection lost evidence or mutated input")
	}
	first := MarkRemoteIssueClosed(model.IssueOpsRecord{}, "first")
	if first.RemoteCompletion.IssueClosedAt != "first" {
		t.Fatal("first close missing")
	}
}

func TestRemoteCompletionProjectsSealedArtifactEvidence(t *testing.T) {
	body := strings.Repeat("x", 4097)
	digest := sha256.Sum256([]byte(body))
	record := model.IssueOpsRecord{Repo: "repo", AISlopCleanVerification: []string{"fallback"}, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "verified"}, Execution: &model.Execution{Workspace: model.Workspace{Root: "worktree"}, Completion: &model.ExecutionCompletion{RemoteArtifactURL: "old", FinalHead: "head"}}}
	section := ProjectRemoteCompletion(record, []CompletionArtifact{{Name: "plan"}, {Name: "verified-execution-loop", Body: body, Present: true}})
	if section.RemoteArtifactURL != "verified" || section.FinalHead != "head" || strings.Join(section.VerificationSummary, ",") != "fallback" || strings.Join(section.MissingArtifacts, ",") != "plan" {
		t.Fatalf("section=%+v", section)
	}
	if len(section.ArtifactManifest) != 1 || section.ArtifactManifest[0].SHA256 != fmt.Sprintf("%x", digest) || section.TuringSummary != strings.Repeat("x", 4096)+"\n\u2026 (\uc808\ub2e8)" {
		t.Fatal("full digest and bounded summary not preserved")
	}
	if CompletionArtifactRoot(record) != "worktree" {
		t.Fatal("worktree must own artifacts")
	}
}
