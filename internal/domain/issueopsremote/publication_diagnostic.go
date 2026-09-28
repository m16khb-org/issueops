package remote

import (
	"strings"

	"issueops/internal/domain/policy"
)

func PublicationFailureDiagnostic(cause error) string {
	if cause == nil {
		return "external operation failed"
	}
	message := strings.TrimSpace(policy.RedactDiagnostic(cause.Error()))
	if len(message) > 4096 {
		message = message[:4096]
	}
	return message
}
