package issueopsbasesync

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type abortEffects struct {
	steps     []string
	mergeCode int
	clearErr  error
}

func (f *abortEffects) AbortMerge(context.Context, string) (int, string) {
	f.steps = append(f.steps, "merge")
	return f.mergeCode, "failed"
}

func (f *abortEffects) ClearResolution(context.Context, string) error {
	f.steps = append(f.steps, "clear")
	return f.clearErr
}

func TestAbortMergeBeforeResolutionClear(t *testing.T) {
	for _, tt := range []struct {
		name, failedStep string
		released         bool
		mergeCode        int
		clearErr         error
		wantSteps        []string
	}{
		{name: "active", wantSteps: []string{"merge"}},
		{name: "released", released: true, wantSteps: []string{"merge", "clear"}},
		{name: "merge failure", released: true, mergeCode: 1, failedStep: "merge_abort", wantSteps: []string{"merge"}},
		{name: "clear failure", released: true, clearErr: errors.New("clear failed"), failedStep: "clear_resolution", wantSteps: []string{"merge", "clear"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			effects := &abortEffects{mergeCode: tt.mergeCode, clearErr: tt.clearErr}
			got, err := Abort(context.Background(), AbortRequest{ID: "id", Root: "root", Released: tt.released}, effects)
			if (err != nil) != (tt.failedStep != "") || got.FailedStep != tt.failedStep || got.Aborted != (tt.failedStep == "") || !reflect.DeepEqual(effects.steps, tt.wantSteps) {
				t.Fatalf("result=%+v err=%v steps=%v, want step=%q steps=%v", got, err, effects.steps, tt.failedStep, tt.wantSteps)
			}
		})
	}
}
