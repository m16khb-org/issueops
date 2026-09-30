package issueops

import (
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

func TestRemoteCompletionProjectsOnlyVerifiedArtifactURL(t *testing.T) {
	record := model.IssueOpsRecord{RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "verified"}, Execution: &model.Execution{Completion: &model.ExecutionCompletion{RemoteArtifactURL: "fallback", FinalHead: "private-head"}}}
	if got := ProjectRemoteCompletion(record); got.RemoteArtifactURL != "verified" || got.ResultBody != "" {
		t.Fatalf("section=%+v", got)
	}
	record.RemoteArtifact = nil
	if got := ProjectRemoteCompletion(record); got.RemoteArtifactURL != "fallback" {
		t.Fatalf("section=%+v", got)
	}
	if got := ProjectRemoteCompletion(model.IssueOpsRecord{}); got != (model.RemoteCompletionSection{}) {
		t.Fatalf("section=%+v", got)
	}
}
