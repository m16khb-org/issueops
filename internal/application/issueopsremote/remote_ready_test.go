package issueopsremote

import (
	"context"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// readyMerger reads back `before` until MarkPullRequestReady runs, then `after`.
type readyMerger struct {
	fakeMerger
	marked int
}

func (f *readyMerger) ReadPullRequestMergeState(context.Context, string, string) (model.RemotePullRequestMergeState, error) {
	if f.marked > 0 {
		return f.after, nil
	}
	return f.before, nil
}

func (f *readyMerger) MarkPullRequestReady(context.Context, string, string) error {
	f.marked++
	return nil
}

func runReady(t *testing.T, merger *readyMerger, confirm bool) (model.RemoteReadyResult, error) {
	t.Helper()
	record := completedMergeRecord()
	service := NewRemoteMergeService(mergeRecordReader{record}, func(string) (port.IssueProviderPullRequestMerger, error) { return merger, nil })
	return service.MarkReady(context.Background(), record.ID, confirm)
}

func TestMarkReadyPreviewsThenClearsTheDraftWithoutMerging(t *testing.T) {
	merger := &readyMerger{fakeMerger: fakeMerger{
		before: openState(func(s *model.RemotePullRequestMergeState) { s.Draft = true }),
		after:  openState(nil),
	}}
	preview, err := runReady(t, merger, false)
	if err != nil || !preview.OK || merger.marked != 0 || !strings.HasSuffix(preview.NextCommand, "--confirm --json") {
		t.Fatalf("preview = %+v, err = %v, marked = %d", preview, err, merger.marked)
	}
	done, err := runReady(t, merger, true)
	if err != nil || !done.MarkedReady || done.Draft || merger.marked != 1 || len(merger.merges) != 0 {
		t.Fatalf("confirm = %+v, err = %v, marked = %d, merges = %d", done, err, merger.marked, len(merger.merges))
	}
}

func TestMarkReadyRefusesAHeadTheCompletionDidNotVerify(t *testing.T) {
	merger := &readyMerger{fakeMerger: fakeMerger{before: openState(func(s *model.RemotePullRequestMergeState) {
		s.Draft, s.HeadOID = true, strings.Repeat("b", 40)
	})}}
	if _, err := runReady(t, merger, true); err == nil || !strings.Contains(err.Error(), "not the completion head") || merger.marked != 0 {
		t.Fatalf("err = %v, marked = %d", err, merger.marked)
	}
}

func TestMarkReadyAnswersAlreadyReadyAndRefusesAClosedArtifact(t *testing.T) {
	ready := &readyMerger{fakeMerger: fakeMerger{before: openState(nil)}}
	if result, err := runReady(t, ready, true); err != nil || !result.AlreadyReady || ready.marked != 0 {
		t.Fatalf("already ready = %+v, err = %v", result, err)
	}
	closed := &readyMerger{fakeMerger: fakeMerger{before: openState(func(s *model.RemotePullRequestMergeState) { s.State, s.Draft = "closed", true })}}
	if _, err := runReady(t, closed, true); err == nil || !strings.Contains(err.Error(), "needs an open PR/MR") || closed.marked != 0 {
		t.Fatalf("closed err = %v", err)
	}
}
