package issueopsintent

import (
	"fmt"
	"issueops/internal/domain/policy"
	"strings"
)

type IntentDraft struct {
	RawRequest, InterpretedIntent                       string
	SuccessCriteria, Constraints, Ambiguities, NonGoals []string
}

// PrepareIntent normalizes the intent contract without observing time or state.
func PrepareIntent(req IntentDraft) (*IntentDraft, error) {
	rawRequest := strings.TrimSpace(req.RawRequest)
	if rawRequest == "" {
		return nil, fmt.Errorf("raw_request is required")
	}
	interpretedIntent := strings.TrimSpace(req.InterpretedIntent)
	if interpretedIntent == "" {
		return nil, fmt.Errorf("interpreted_intent is required")
	}
	if interpretedIntent == rawRequest {
		return nil, fmt.Errorf("interpreted_intent must differ from raw_request")
	}
	if !MateriallyDifferentIntent(rawRequest, interpretedIntent) {
		return nil, fmt.Errorf("interpreted_intent must materially differ from raw_request")
	}
	successCriteria := CleanTextValues(req.SuccessCriteria)
	if len(successCriteria) == 0 {
		return nil, fmt.Errorf("success_criteria is required")
	}
	return &IntentDraft{
		RawRequest:        policy.RedactFreeform(rawRequest),
		InterpretedIntent: policy.RedactFreeform(interpretedIntent),
		SuccessCriteria:   successCriteria,
		Constraints:       CleanTextValues(req.Constraints),
		Ambiguities:       CleanTextValues(req.Ambiguities),
		NonGoals:          CleanTextValues(req.NonGoals),
	}, nil
}
