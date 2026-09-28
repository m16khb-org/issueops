package issueopsbasesync

import "fmt"

type ResolutionState struct {
	ExecutionPresent     bool
	CompletionPresent    bool
	ResolutionPresent    bool
	LeaseGeneration      uint64
	CompletionGeneration uint64
}

type ResolutionGenerations struct {
	Lease      uint64
	Completion uint64
}

func ValidateEventAppend(executionPresent bool) error {
	if !executionPresent {
		return fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	return nil
}

func BeginResolution(state ResolutionState) (ResolutionGenerations, error) {
	if !state.ExecutionPresent || !state.CompletionPresent || state.ResolutionPresent {
		return ResolutionGenerations{}, fmt.Errorf("execution sync-base resolution state changed before sealing")
	}
	return ResolutionGenerations{Lease: state.LeaseGeneration, Completion: state.CompletionGeneration}, nil
}

func ValidateResolutionClear(state ResolutionState) error {
	if !state.ExecutionPresent || !state.ResolutionPresent {
		return fmt.Errorf("execution sync-base resolution authority is absent")
	}
	return nil
}
