package issueopsremote

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
		diagnostic = diagnostic[:maxBytes]
	}
	return diagnostic
}
