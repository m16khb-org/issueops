package remote

import (
	"strings"

	policy "issueops/internal/domain/policy"
)

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
