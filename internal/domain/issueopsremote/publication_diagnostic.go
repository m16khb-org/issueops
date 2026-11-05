package issueopsremote

import (
	"strings"

	"issueops/internal/domain/policy"
)

// PublicationFailureDiagnostic renders cause as a redacted diagnostic truncated to 4096 bytes for a failed external publication. A nil cause yields a generic message.
func PublicationFailureDiagnostic(cause error) string {
	if cause == nil {
		return "external operation failed"
	}
	message := strings.TrimSpace(policy.RedactDiagnostic(cause.Error()))
	if len(message) > 4096 {
		message = policy.TruncateBytes(message, 4096)
	}
	return message
}
