package issueopsbasesync

import "fmt"

// ResolutionState is the lifecycle record's execution, completion, and
// sync-base resolution presence and generations before a resolution transition.
type ResolutionState struct {
	ExecutionPresent     bool
	CompletionPresent    bool
	ResolutionPresent    bool
	LeaseGeneration      uint64
	CompletionGeneration uint64
}

// ResolutionGenerations are the lease and completion generations a new
// resolution is sealed under.
type ResolutionGenerations struct {
	Lease      uint64
	Completion uint64
}

// ValidateEventAppend fails when the execution is not prepared, since a
// sync-base event can only be appended to a prepared execution.
func ValidateEventAppend(executionPresent bool) error {
	if !executionPresent {
		return fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	return nil
}

// BeginResolution returns the generations a new resolution is sealed under. It
// fails when the execution or completion is missing or a resolution already
// exists, meaning the state changed since it was read.
func BeginResolution(state ResolutionState) (ResolutionGenerations, error) {
	if !state.ExecutionPresent || !state.CompletionPresent || state.ResolutionPresent {
		return ResolutionGenerations{}, fmt.Errorf("execution sync-base resolution state changed before sealing")
	}
	return ResolutionGenerations{Lease: state.LeaseGeneration, Completion: state.CompletionGeneration}, nil
}

// ValidateResolutionClear fails when the execution or the resolution to clear
// is absent.
func ValidateResolutionClear(state ResolutionState) error {
	if !state.ExecutionPresent || !state.ResolutionPresent {
		return fmt.Errorf("execution sync-base resolution authority is absent")
	}
	return nil
}
