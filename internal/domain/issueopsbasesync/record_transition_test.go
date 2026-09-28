package issueopsbasesync

import "testing"

func TestRecordTransitionEligibility(t *testing.T) {
	if err := ValidateEventAppend(false); err == nil {
		t.Fatal("event appended without execution")
	}
	if err := ValidateEventAppend(true); err != nil {
		t.Fatal(err)
	}
	for _, state := range []ResolutionState{
		{},
		{ExecutionPresent: true},
		{ExecutionPresent: true, CompletionPresent: true, ResolutionPresent: true},
	} {
		if _, err := BeginResolution(state); err == nil {
			t.Fatalf("invalid resolution accepted: %+v", state)
		}
	}
	got, err := BeginResolution(ResolutionState{ExecutionPresent: true, CompletionPresent: true, LeaseGeneration: 5, CompletionGeneration: 4})
	if err != nil || got != (ResolutionGenerations{Lease: 5, Completion: 4}) {
		t.Fatalf("generations=%+v err=%v", got, err)
	}
	if err := ValidateResolutionClear(ResolutionState{ExecutionPresent: true}); err == nil {
		t.Fatal("absent resolution cleared")
	}
	if err := ValidateResolutionClear(ResolutionState{ExecutionPresent: true, ResolutionPresent: true}); err != nil {
		t.Fatal(err)
	}
}
