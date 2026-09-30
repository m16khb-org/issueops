package issueops

import (
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func cleanupStatusRecordForDomain() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "cycle", Phase: model.IssueOpsPhaseDone, Branch: "1-cleanup", WorktreePath: "/worktree", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/1", Labels: []string{"ready"}, Assignees: []string{"owner"}}}
}

func TestCleanupStatusUsesObservedFactsAndStableRequirementPolarity(t *testing.T) {
	record := cleanupStatusRecordForDomain()
	facts := CleanupStatusObservation{WorktreeExists: true, Branch: "1-cleanup", Remote: "origin"}
	ready := BuildCleanupStatus(record, model.IssueOpsCleanupStatusRequest{Merged: true}, facts)
	if !ready.Ready || len(ready.Missing) != 0 || len(ready.Choices) != 3 {
		t.Fatalf("ready: %+v", ready)
	}
	cases := []struct {
		name    string
		change  func(*CleanupStatusObservation)
		missing string
		warning string
	}{
		{"missing worktree", func(f *CleanupStatusObservation) { f.WorktreeExists = false }, "worktree_exists", ""},
		{"dirty", func(f *CleanupStatusObservation) { f.StatusOutput = "?? file" }, "worktree_clean", ""},
		{"git failure", func(f *CleanupStatusObservation) { f.StatusCode = 1; f.StatusError = " denied " }, "worktree_git_status", "denied"},
		{"branch mismatch", func(f *CleanupStatusObservation) { f.Branch = "other" }, "branch_match", ""},
		{"detached", func(f *CleanupStatusObservation) { f.Branch = "" }, "branch", ""},
		{"no remote", func(f *CleanupStatusObservation) { f.Remote = "" }, "remote_branch_check_unavailable", ""},
		{"remote failure", func(f *CleanupStatusObservation) { f.RemoteCode = 1; f.RemoteError = " offline " }, "remote_branch_check_failed", "offline"},
		{"remote present", func(f *CleanupStatusObservation) { f.RemoteOutput = "sha refs/heads/1-cleanup" }, "remote_branch_absent", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := facts
			tc.change(&observed)
			got := BuildCleanupStatus(record, model.IssueOpsCleanupStatusRequest{Merged: true}, observed)
			if got.Ready || !reflect.DeepEqual(got.Missing, []string{tc.missing}) {
				t.Fatalf("status: %+v", got)
			}
			if tc.warning != "" && !reflect.DeepEqual(got.Warnings, []string{tc.warning}) {
				t.Fatalf("warnings: %v", got.Warnings)
			}
		})
	}
}

func TestCleanupStatusRequiresStructuralAndRemoteEvidence(t *testing.T) {
	record := cleanupStatusRecordForDomain()
	record.Phase = model.IssueOpsPhasePlan
	record.WorktreePath = ""
	record.RemoteArtifact.Labels = []string{" ", "bad\x00label"}
	record.RemoteArtifact.Assignees = []string{"\x00"}
	record.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: "child"}}
	got := BuildCleanupStatus(record, model.IssueOpsCleanupStatusRequest{}, CleanupStatusObservation{})
	want := []string{"child_tasks_closed", "pr_phase", "remote_artifact_assignees", "remote_artifact_labels", "remote_artifact_merged", "worktree_path"}
	if !reflect.DeepEqual(got.Missing, want) {
		t.Fatalf("missing=%v", got.Missing)
	}
	if CleanupStatusNeedsMergeReadback(record, true) {
		t.Fatal("ineligible record requested provider observation")
	}
	record = cleanupStatusRecordForDomain()
	if !CleanupStatusNeedsMergeReadback(record, true) || CleanupStatusNeedsMergeReadback(record, false) {
		t.Fatal("merge readback eligibility incorrect")
	}
}

func TestFinalizeCleanupStatusDoesNotMutateInput(t *testing.T) {
	values := []string{"b", "", "a", "b"}
	got := FinalizeCleanupStatus(model.IssueOpsCleanupStatus{Missing: values, Warnings: values})
	if !reflect.DeepEqual(got.Missing, []string{"a", "b"}) || !reflect.DeepEqual(got.Warnings, []string{"a", "b"}) {
		t.Fatalf("finalize: %+v", got)
	}
	if !reflect.DeepEqual(values, []string{"b", "", "a", "b"}) {
		t.Fatal("input changed")
	}
}
