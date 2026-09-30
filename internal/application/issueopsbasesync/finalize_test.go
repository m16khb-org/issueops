package issueopsbasesync

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"issueops/internal/contract/issueops"
)

type finalizeEffects struct {
	steps       []string
	unmerged    []string
	checkCode   int
	commitCode  int
	headCode    int
	pushCode    int
	appendError error
	event       issueops.ExecutionSyncBaseEvent
}

func (f *finalizeEffects) ConflictCount(context.Context, string) int {
	f.steps = append(f.steps, "conflict-count")
	return 2
}

func (f *finalizeEffects) UnmergedPaths(context.Context, string) []string {
	f.steps = append(f.steps, "unmerged")
	return f.unmerged
}

func (f *finalizeEffects) CheckStaged(context.Context, string) (int, string) {
	f.steps = append(f.steps, "check")
	return f.checkCode, "check failed"
}

func (f *finalizeEffects) Commit(context.Context, string) (int, string) {
	f.steps = append(f.steps, "commit")
	return f.commitCode, "commit failed"
}

func (f *finalizeEffects) Head(context.Context, string) (int, string) {
	f.steps = append(f.steps, "head")
	if f.headCode != 0 {
		return f.headCode, "head failed"
	}
	return 0, "merge-head"
}

func (f *finalizeEffects) Now() string { return "now" }

func (f *finalizeEffects) Push(_ context.Context, _, _ string) (int, string) {
	f.steps = append(f.steps, "push")
	return f.pushCode, "push failed"
}

func (f *finalizeEffects) AppendEvent(_ context.Context, _ string, event issueops.ExecutionSyncBaseEvent) error {
	f.steps = append(f.steps, "event")
	f.event = event
	return f.appendError
}

func TestFinalizeChecksConflictsBeforeCommitAndPush(t *testing.T) {
	request := FinalizeRequest{Push: PushRequest{ID: "id", Root: "root", Branch: "topic", Mode: "finalize", BaseBranch: "main", BaseOID: "base", Actor: "owner"}}
	for _, tt := range []struct {
		name, failedStep string
		unmerged         []string
		checkCode        int
		commitCode       int
		headCode         int
		pushCode         int
		appendError      error
		wantSteps        []string
		wantDirect       bool
		wantMerged       bool
		wantPushed       bool
	}{
		{name: "success", wantSteps: []string{"conflict-count", "unmerged", "check", "commit", "head", "push", "event"}, wantMerged: true, wantPushed: true},
		{name: "unmerged", unmerged: []string{"a.go"}, wantSteps: []string{"conflict-count", "unmerged"}, wantDirect: true},
		{name: "markers", checkCode: 1, wantSteps: []string{"conflict-count", "unmerged", "check"}, wantDirect: true},
		{name: "commit failure", commitCode: 1, failedStep: "merge_commit", wantSteps: []string{"conflict-count", "unmerged", "check", "commit"}},
		{name: "head failure", headCode: 1, failedStep: "head", wantSteps: []string{"conflict-count", "unmerged", "check", "commit", "head"}},
		{name: "push failure", pushCode: 1, failedStep: "push", wantSteps: []string{"conflict-count", "unmerged", "check", "commit", "head", "push"}, wantMerged: true},
		{name: "event failure", appendError: errors.New("record failed"), failedStep: "record_event", wantSteps: []string{"conflict-count", "unmerged", "check", "commit", "head", "push", "event"}, wantMerged: true, wantPushed: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			effects := &finalizeEffects{unmerged: tt.unmerged, checkCode: tt.checkCode, commitCode: tt.commitCode, headCode: tt.headCode, pushCode: tt.pushCode, appendError: tt.appendError}
			got, err := Finalize(context.Background(), request, effects)
			if (err != nil) != (tt.wantDirect || tt.failedStep != "") || got.FailedStep != tt.failedStep || got.DirectError != tt.wantDirect || got.Merged != tt.wantMerged || got.Pushed != tt.wantPushed || !reflect.DeepEqual(effects.steps, tt.wantSteps) {
				t.Fatalf("result=%+v err=%v steps=%v", got, err, effects.steps)
			}
			if tt.wantPushed && effects.event.ConflictFiles != 2 {
				t.Fatalf("event conflict count=%d", effects.event.ConflictFiles)
			}
		})
	}
}
