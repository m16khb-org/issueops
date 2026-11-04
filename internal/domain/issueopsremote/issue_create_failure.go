package issueopsremote

import (
	"strings"

	policy "issueops/internal/domain/policy"
)

// IssueCreateFailure renders err as a redacted diagnostic truncated to 2048 bytes for the issue-create failure record. A nil err yields "".
func IssueCreateFailure(err error) string {
	if err == nil {
		return ""
	}
	const maxBytes = 2048
	diagnostic := policy.RedactDiagnostic(strings.TrimSpace(err.Error()))
	if len(diagnostic) > maxBytes {
		diagnostic = policy.TruncateBytes(diagnostic, maxBytes)
	}
	return diagnostic
}
