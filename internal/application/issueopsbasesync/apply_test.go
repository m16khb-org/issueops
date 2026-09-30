package issueopsbasesync

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"issueops/internal/contract/issueops"
)

type applyEffects struct {
	steps           []string
	conflicts       []string
	unmerged        []string
	predictErr      error
	mergeCode       int
	resolutionError error
	headCode        int
	pushCode        int
	event           issueops.ExecutionSyncBaseEvent
}

func (f *applyEffects) PredictConflicts(context.Context, string, string, string) ([]string, error) {
	f.steps = append(f.steps, "predict")
	return f.conflicts, f.predictErr
}

func (f *applyEffects) Merge(context.Context, string, string) (int, string) {
	f.steps = append(f.steps, "merge")
	return f.mergeCode, "merge failed"
}

func (f *applyEffects) UnmergedPaths(context.Context, string) []string {
	f.steps = append(f.steps, "unmerged")
	return f.unmerged
}

func (f *applyEffects) StartResolution(_ context.Context, _ string, _ []string) error {
	f.steps = append(f.steps, "resolution")
	return f.resolutionError
}

func (f *applyEffects) AbortMerge(context.Context, string) (int, string) {
	f.steps = append(f.steps, "abort")
	return 0, ""
}

func (f *applyEffects) Head(context.Context, string) (int, string) {
	f.steps = append(f.steps, "head")
	if f.headCode != 0 {
		return f.headCode, "head failed"
	}
	return 0, "merge-head"
}

func (f *applyEffects) Now() string { return "now" }

func (f *applyEffects) Push(context.Context, string, string) (int, string) {
	f.steps = append(f.steps, "push")
	return f.pushCode, "push failed"
}

func (f *applyEffects) AppendEvent(_ context.Context, _ string, event issueops.ExecutionSyncBaseEvent) error {
	f.steps = append(f.steps, "event")
	f.event = event
	return nil
}

func TestApplyMergeAndPushSequence(t *testing.T) {
	base := ApplyRequest{Push: PushRequest{ID: "id", Root: "root", Branch: "topic", Mode: "apply", BaseBranch: "main", BaseOID: "base", Actor: "owner"}, WorkOID: "work"}
	for _, tt := range []struct {
		name, failedStep                   string
		mergeNeeded, released              bool
		conflicts, unmerged                []string
		predictErr                         error
		mergeCode                          int
		resolutionError                    error
		pushCode                           int
		wantSteps                          []string
		wantPaused, wantMerged, wantPushed bool
	}{
		{name: "push only", wantSteps: []string{"head", "push", "event"}, wantPushed: true},
		{name: "merge success", mergeNeeded: true, wantSteps: []string{"predict", "merge", "head", "push", "event"}, wantMerged: true, wantPushed: true},
		{name: "conflict pause", mergeNeeded: true, released: true, conflicts: []string{"a.go"}, mergeCode: 1, wantSteps: []string{"predict", "merge", "resolution"}, wantPaused: true},
		{name: "actual conflict fallback", mergeNeeded: true, released: true, mergeCode: 1, unmerged: []string{"b.go"}, wantSteps: []string{"predict", "merge", "unmerged", "resolution"}, wantPaused: true},
		{name: "merge failure", mergeNeeded: true, mergeCode: 1, failedStep: "merge", wantSteps: []string{"predict", "merge", "unmerged"}},
		{name: "prediction failure", mergeNeeded: true, predictErr: errors.New("unsupported"), failedStep: "merge_tree", wantSteps: []string{"predict"}},
		{name: "resolution failure", mergeNeeded: true, released: true, conflicts: []string{"a.go"}, mergeCode: 1, resolutionError: errors.New("record failed"), failedStep: "record_resolution", wantSteps: []string{"predict", "merge", "resolution", "abort"}},
		{name: "push failure", pushCode: 1, failedStep: "push", wantSteps: []string{"head", "push"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			effects := &applyEffects{conflicts: tt.conflicts, unmerged: tt.unmerged, predictErr: tt.predictErr, mergeCode: tt.mergeCode, resolutionError: tt.resolutionError, pushCode: tt.pushCode}
			request := base
			request.MergeNeeded, request.Released = tt.mergeNeeded, tt.released
			got, err := Apply(context.Background(), request, effects)
			if (err != nil) != (tt.failedStep != "") || got.FailedStep != tt.failedStep || got.ConflictPaused != tt.wantPaused || got.Merged != tt.wantMerged || got.Pushed != tt.wantPushed || !reflect.DeepEqual(effects.steps, tt.wantSteps) {
				t.Fatalf("result=%+v err=%v steps=%v", got, err, effects.steps)
			}
			if tt.wantPushed && effects.event.Mode != "apply" {
				t.Fatalf("event=%+v", effects.event)
			}
		})
	}
}
