package issueops

import (
	"errors"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func remoteCleanupRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "io-test", Repo: "/repo", Branch: "123-work", Phase: model.IssueOpsPhaseDone,
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: "main"},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/1"}}
}

func TestRemoteBranchCleanupCollectsAllAggregateGates(t *testing.T) {
	record := remoteCleanupRecord()
	record.Branch = "main"
	record.Phase = model.IssueOpsPhasePR
	record.Execution = &model.Execution{Lease: model.WriteLease{Status: model.LeaseStatusActive}}
	record.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: "https://github.com/acme/repo/issues/2"}}
	record.RemoteArtifact = nil
	_, got := BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, CleanupRemoteBranchTargets(record), CleanupRemoteBranchObservation{RemoteError: errors.New("transport")})
	want := []string{"branch_name_revalidated", "branch_not_base", "phase_done", "lease_released", "child_tasks_closed", "remote_artifact_present", "remote_branch_readable"}
	if got.OK || !reflect.DeepEqual(got.Missing, want) {
		t.Fatalf("gates=%+v want=%v", got, want)
	}
	record.Execution.Workspace.Branch = " 123-work "
	if got := CleanupRemoteBranchTargets(record).Branch; got != "123-work" {
		t.Fatalf("workspace target=%q", got)
	}
}

func TestRemoteBranchCleanupRequiresPositiveSupersedeEvidence(t *testing.T) {
	record := remoteCleanupRecord()
	for _, failedMerge := range []bool{false, true} {
		facts := CleanupRemoteBranchObservation{MergeConfigured: true, Head: model.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "old"}, RemoteOID: "new"}
		if failedMerge {
			facts.MergeError = errors.New("not merged")
		}
		_, got := BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{SupersededBy: "https://github.com/acme/repo/pull/2"}, CleanupRemoteBranchTargets(record), facts)
		if got.OK || got.SupersededBy != "" {
			t.Fatalf("missing positive evidence accepted: failedMerge=%v got=%+v", failedMerge, got)
		}
	}
}

func TestRemoteBranchCleanupKeepsSquashAndAncestryEvidenceDistinct(t *testing.T) {
	record := remoteCleanupRecord()
	facts := CleanupRemoteBranchObservation{MergeConfigured: true, Head: model.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: "ABC"}, RemoteOID: "abc"}
	if CleanupRemoteBranchNeedsTipEvidence(facts) {
		t.Fatal("squash head requires extra proof")
	}
	_, got := BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, CleanupRemoteBranchTargets(record), facts)
	if !got.OK || got.RemoteTipReachedBase {
		t.Fatalf("squash evidence=%+v", got)
	}
	facts.RemoteOID = "different"
	facts.TipReachedBase = true
	_, got = BuildCleanupRemoteBranchPreview(record, model.CleanupRemoteBranchRequest{}, CleanupRemoteBranchTargets(record), facts)
	if !got.OK || !got.RemoteTipReachedBase {
		t.Fatalf("ancestry evidence=%+v", got)
	}
}
