package issueopsbasesync

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"issueops/internal/contract/issueops"
)

type pushEffects struct {
	steps       []string
	pushCode    int
	appendError error
	event       issueops.ExecutionSyncBaseEvent
}

func (f *pushEffects) Push(_ context.Context, root, refspec string) (int, string) {
	f.steps = append(f.steps, "push:"+root+":"+refspec)
	return f.pushCode, "failed"
}

func (f *pushEffects) AppendEvent(_ context.Context, id string, event issueops.ExecutionSyncBaseEvent) error {
	f.steps = append(f.steps, "event:"+id)
	f.event = event
	return f.appendError
}

func TestPushAndRecordWritesEventOnlyAfterSuccessfulPush(t *testing.T) {
	request := PushRequest{ID: "id", Root: "root", Branch: "topic", Mode: "apply", BaseBranch: "main", BaseOID: "base", MergeCommit: "merge", ConflictFiles: 2, Actor: "owner", At: "now"}
	refspec := "refs/heads/topic:refs/heads/topic"
	for _, tt := range []struct {
		name       string
		pushCode   int
		appendErr  error
		wantSteps  []string
		wantResult PushResult
	}{
		{name: "success", wantSteps: []string{"push:root:" + refspec, "event:id"}, wantResult: PushResult{Pushed: true}},
		{name: "push failure", pushCode: 1, wantSteps: []string{"push:root:" + refspec}, wantResult: PushResult{PushRetryRequired: true, FailedStep: "push"}},
		{name: "event failure", appendErr: errors.New("record failed"), wantSteps: []string{"push:root:" + refspec, "event:id"}, wantResult: PushResult{Pushed: true, FailedStep: "record_event"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			effects := &pushEffects{pushCode: tt.pushCode, appendError: tt.appendErr}
			got, err := PushAndRecord(context.Background(), request, effects)
			if (err != nil) != (tt.wantResult.FailedStep != "") || got != tt.wantResult || !reflect.DeepEqual(effects.steps, tt.wantSteps) {
				t.Fatalf("result=%+v err=%v steps=%v, want result=%+v steps=%v", got, err, effects.steps, tt.wantResult, tt.wantSteps)
			}
			if tt.wantResult.Pushed && effects.event != (issueops.ExecutionSyncBaseEvent{Mode: "apply", BaseBranch: "main", BaseOID: "base", MergeCommit: "merge", ConflictFiles: 2, Actor: "owner", At: "now"}) {
				t.Fatalf("event=%+v", effects.event)
			}
		})
	}
}
