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

func TestMissingMergeStateGatesPreservesModeRequirements(t *testing.T) {
	for _, test := range []struct {
		name  string
		facts MergeStateFacts
		want  []string
	}{
		{name: "preview with merge", facts: MergeStateFacts{Mode: "preview", MergeInProgress: true}, want: []string{}},
		{name: "apply with merge and dirty tree", facts: MergeStateFacts{Mode: "apply", MergeInProgress: true, TrackedDirty: true}, want: []string{"merge_state_clean", "worktree_clean"}},
		{name: "apply clean", facts: MergeStateFacts{Mode: "apply"}, want: []string{}},
		{name: "finalize without merge", facts: MergeStateFacts{Mode: "finalize"}, want: []string{"merge_in_progress"}},
		{name: "abort without merge", facts: MergeStateFacts{Mode: "abort"}, want: []string{"merge_in_progress"}},
		{name: "finalize with merge", facts: MergeStateFacts{Mode: "finalize", MergeInProgress: true}, want: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := MissingMergeStateGates(test.facts); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("missing=%v, want %v", got, test.want)
			}
		})
	}
}
