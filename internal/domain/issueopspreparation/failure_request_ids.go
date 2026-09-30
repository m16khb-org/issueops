package issueopspreparation

import "strings"

type FailureRequestIDFacts struct {
	SealedDispatch        string
	SealedPrompt          string
	CallPhase             string
	ObservedDispatch      string
	ObservedOrchestration string
	DispatchValid         bool
	OrchestrationValid    bool
}

func AdoptFailureRequestIDs(facts FailureRequestIDFacts) (dispatch, prompt string) {
	dispatch = strings.TrimSpace(facts.SealedDispatch)
	prompt = strings.TrimSpace(facts.SealedPrompt)
	if facts.CallPhase == "terminal_send" {
		return adoptFailureRequestID(dispatch, facts.ObservedDispatch, facts.DispatchValid),
			adoptFailureRequestID(prompt, facts.ObservedOrchestration, facts.OrchestrationValid)
	}
	return adoptFailureRequestID(dispatch, facts.ObservedOrchestration, facts.OrchestrationValid), prompt
}

func adoptFailureRequestID(sealed, observed string, valid bool) string {
	sealed = strings.TrimSpace(sealed)
	observed = strings.TrimSpace(observed)
	if !valid {
		return sealed
	}
	if sealed == "" || sealed == observed {
		return observed
	}
	return sealed
}
