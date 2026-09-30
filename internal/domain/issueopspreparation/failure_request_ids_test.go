package issueopspreparation

import "testing"

func TestAdoptFailureRequestIDsPreservesSealedIdentity(t *testing.T) {
	for _, tt := range []struct {
		name                     string
		facts                    FailureRequestIDFacts
		wantDispatch, wantPrompt string
	}{
		{name: "terminal send", facts: FailureRequestIDFacts{CallPhase: "terminal_send", ObservedDispatch: "dispatch", ObservedOrchestration: "prompt", DispatchValid: true, OrchestrationValid: true}, wantDispatch: "dispatch", wantPrompt: "prompt"},
		{name: "other phase", facts: FailureRequestIDFacts{ObservedOrchestration: "orchestration", OrchestrationValid: true}, wantDispatch: "orchestration"},
		{name: "invalid observed", facts: FailureRequestIDFacts{SealedDispatch: "sealed", ObservedOrchestration: "invalid", OrchestrationValid: false}, wantDispatch: "sealed"},
		{name: "conflicting observed", facts: FailureRequestIDFacts{SealedDispatch: "sealed", ObservedOrchestration: "other", OrchestrationValid: true}, wantDispatch: "sealed"},
		{name: "same observed", facts: FailureRequestIDFacts{SealedDispatch: "sealed", ObservedOrchestration: "sealed", OrchestrationValid: true}, wantDispatch: "sealed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dispatch, prompt := AdoptFailureRequestIDs(tt.facts)
			if dispatch != tt.wantDispatch || prompt != tt.wantPrompt {
				t.Fatalf("dispatch=%q prompt=%q, want %q %q", dispatch, prompt, tt.wantDispatch, tt.wantPrompt)
			}
		})
	}
}
