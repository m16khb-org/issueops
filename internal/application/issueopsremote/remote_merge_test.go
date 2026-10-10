package issueopsremote

import (
	"context"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

const mergeTestHead = "0123456789abcdef0123456789abcdef01234567"

type mergeRecordReader struct{ record model.IssueOpsRecord }

func (r mergeRecordReader) Read(context.Context, string) (model.IssueOpsRecord, error) {
	return r.record, nil
}

type fakeMerger struct {
	before, after model.RemotePullRequestMergeState
	merges        []port.IssueProviderPullRequestMergeRequest
}

func (f *fakeMerger) ReadPullRequestMergeState(context.Context, string, string) (model.RemotePullRequestMergeState, error) {
	return f.before, nil
}

func (f *fakeMerger) MergePullRequest(_ context.Context, req port.IssueProviderPullRequestMergeRequest) (model.RemotePullRequestMergeState, error) {
	f.merges = append(f.merges, req)
	return f.after, nil
}

func completedMergeRecord() model.IssueOpsRecord {
	url := "https://github.com/acme/repo/pull/7"
	return model.IssueOpsRecord{
		ID: "io-merge", Repo: "/repo", Phase: model.IssueOpsPhaseDone,
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: url},
		Execution: &model.Execution{
			Lease:      model.WriteLease{Generation: 2, Status: model.LeaseStatusReleased},
			Completion: &model.ExecutionCompletion{Generation: 2, FinalHead: mergeTestHead, RemoteArtifactURL: url},
		},
	}
}

func openState(mutate func(*model.RemotePullRequestMergeState)) model.RemotePullRequestMergeState {
	state := model.RemotePullRequestMergeState{State: "open", HeadOID: mergeTestHead, BaseBranch: "main", Checks: model.RemoteChecksPassing}
	if mutate != nil {
		mutate(&state)
	}
	return state
}

func runMerge(t *testing.T, record model.IssueOpsRecord, merger *fakeMerger, method string, confirm bool) (model.RemoteMergeResult, error) {
	t.Helper()
	service := NewRemoteMergeService(mergeRecordReader{record}, func(string) (port.IssueProviderPullRequestMerger, error) { return merger, nil })
	return service.Merge(context.Background(), record.ID, method, confirm)
}

func TestRemoteMergeRefusesBlockersWithoutMutation(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state model.RemotePullRequestMergeState
		code  string
	}{
		{"failing checks", openState(func(s *model.RemotePullRequestMergeState) {
			s.Checks, s.Failing = model.RemoteChecksFailing, []string{"verify"}
		}), model.RemoteMergeBlockerChecksFailing},
		{"pending checks", openState(func(s *model.RemotePullRequestMergeState) {
			s.Checks, s.Pending = model.RemoteChecksPending, []string{"verify"}
		}), model.RemoteMergeBlockerChecksPending},
		{"commits after completion", openState(func(s *model.RemotePullRequestMergeState) { s.HeadOID = "ffff" }), model.RemoteMergeBlockerHeadMismatch},
		{"conflict", openState(func(s *model.RemotePullRequestMergeState) { s.Conflict = true }), model.RemoteMergeBlockerConflict},
		{"closed", openState(func(s *model.RemotePullRequestMergeState) { s.State = "closed" }), model.RemoteMergeBlockerNotOpen},
		{"provider block", openState(func(s *model.RemotePullRequestMergeState) { s.Blocked = "not_approved" }), model.RemoteMergeBlocked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merger := &fakeMerger{before: tc.state}
			preview, err := runMerge(t, completedMergeRecord(), merger, "", false)
			if err != nil || preview.OK || len(preview.Blockers) != 1 || preview.Blockers[0].Code != tc.code || preview.NextCommand != "" {
				t.Fatalf("preview = %+v err=%v", preview, err)
			}
			confirmed, err := runMerge(t, completedMergeRecord(), merger, "", true)
			if err == nil || !strings.Contains(err.Error(), tc.code) || confirmed.Merged || len(merger.merges) != 0 {
				t.Fatalf("confirm must refuse before mutation: result=%+v err=%v merges=%d", confirmed, err, len(merger.merges))
			}
		})
	}
}

func TestRemoteMergeSquashesDraftPinnedToCompletedHead(t *testing.T) {
	merger := &fakeMerger{
		before: openState(func(s *model.RemotePullRequestMergeState) { s.Draft = true }),
		after:  openState(func(s *model.RemotePullRequestMergeState) { s.State, s.MergeCommitOID = "merged", "abc" }),
	}
	preview, err := runMerge(t, completedMergeRecord(), merger, "", false)
	if err != nil || !preview.OK || preview.Method != model.RemoteMergeMethodSquash || !strings.Contains(preview.NextCommand, "--method squash --confirm") || len(merger.merges) != 0 {
		t.Fatalf("preview = %+v err=%v merges=%d", preview, err, len(merger.merges))
	}
	result, err := runMerge(t, completedMergeRecord(), merger, "", true)
	if err != nil || !result.OK || !result.Merged || !result.MarkedReady || result.MergeCommitOID != "abc" || !strings.Contains(result.NextCommand, "cleanup status") {
		t.Fatalf("result = %+v err=%v", result, err)
	}
	if len(merger.merges) != 1 || merger.merges[0].HeadOID != mergeTestHead || !merger.merges[0].MarkReady || merger.merges[0].Method != model.RemoteMergeMethodSquash {
		t.Fatalf("merge request = %+v", merger.merges)
	}
}

func TestRemoteMergeReportsUnverifiedMerge(t *testing.T) {
	merger := &fakeMerger{before: openState(nil), after: openState(nil)}
	result, err := runMerge(t, completedMergeRecord(), merger, model.RemoteMergeMethodMerge, true)
	if err == nil || result.Merged || result.OK {
		t.Fatalf("an open readback after merge must not be reported as merged: %+v err=%v", result, err)
	}
}

func TestRemoteMergeIsIdempotentOnMergedArtifact(t *testing.T) {
	merger := &fakeMerger{before: openState(func(s *model.RemotePullRequestMergeState) { s.State, s.HeadOID = "merged", "ffff" })}
	result, err := runMerge(t, completedMergeRecord(), merger, "", true)
	if err != nil || !result.OK || !result.AlreadyMerged || len(result.Blockers) != 0 || len(merger.merges) != 0 {
		t.Fatalf("result = %+v err=%v", result, err)
	}
}

func TestRemoteMergeRequiresCompletedCycle(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.IssueOpsRecord)
	}{
		{"pr phase", func(r *model.IssueOpsRecord) { r.Phase = model.IssueOpsPhasePR }},
		{"no completion", func(r *model.IssueOpsRecord) { r.Execution.Completion = nil }},
		{"active lease", func(r *model.IssueOpsRecord) { r.Execution.Lease.Status = model.LeaseStatusActive }},
		{"no artifact", func(r *model.IssueOpsRecord) { r.RemoteArtifact = nil }},
		{"artifact drift", func(r *model.IssueOpsRecord) {
			r.Execution.Completion.RemoteArtifactURL = "https://github.com/acme/repo/pull/8"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := completedMergeRecord()
			tc.mutate(&record)
			merger := &fakeMerger{before: openState(nil)}
			if _, err := runMerge(t, record, merger, "", false); err == nil {
				t.Fatal("merge-pr must refuse a cycle that is not completed")
			}
		})
	}
	if _, err := runMerge(t, completedMergeRecord(), &fakeMerger{}, "fast-forward", false); err == nil {
		t.Fatal("unknown merge method must be refused")
	}
}
