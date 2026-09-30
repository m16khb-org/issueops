package issueops

import (
	"strings"

	model "issueops/internal/contract/issueops"
)

func IntentMissing(record model.IssueOpsRecord) []string {
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
