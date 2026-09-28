package issueopsbasesync

import (
	"reflect"
	"testing"
)

func TestMissingRecordGatesPreservesBaseSyncEligibilityOrder(t *testing.T) {
	for _, test := range []struct {
		name  string
		facts RecordGateFacts
		want  []string
	}{
		{name: "execution absent", want: []string{"execution_prepared"}},
		{name: "active execution", facts: RecordGateFacts{ExecutionPresent: true, LeaseActive: true, CompletionPresent: false, PendingIntentAbsent: true, BaseBranchPresent: true}, want: []string{}},
		{name: "released missing evidence", facts: RecordGateFacts{ExecutionPresent: true}, want: []string{"completion_present", "remote_artifact_present", "pending_intent_absent", "base_branch_present"}},
		{name: "released incomplete completion", facts: RecordGateFacts{ExecutionPresent: true, CompletionPresent: true, RemoteArtifactPresent: true, PendingIntentAbsent: true, BaseBranchPresent: true}, want: []string{"current_completion_generation_present"}},
		{name: "released ready", facts: RecordGateFacts{ExecutionPresent: true, CompletionPresent: true, CompletionGeneration: 2, RemoteArtifactPresent: true, PendingIntentAbsent: true, BaseBranchPresent: true}, want: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := MissingRecordGates(test.facts); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("missing=%v, want %v", got, test.want)
			}
		})
	}
}
