package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

// IntentMissing lists the intent contract fields still absent from record:
// the contract itself, the raw request, the interpreted intent, and usable
// success criteria.
func IntentMissing(record model.Record) []string {
	if record.Intent == nil {
		return []string{"intent_contract"}
	}
	missing := []string{}
	if strings.TrimSpace(record.Intent.RawRequest) == "" {
		missing = append(missing, "raw_request")
	}
	if strings.TrimSpace(record.Intent.InterpretedIntent) == "" {
		missing = append(missing, "interpreted_intent")
	}
	if !hasUsableEvidence(record.Intent.SuccessCriteria) {
		missing = append(missing, "success_criteria")
	}
	return missing
}
