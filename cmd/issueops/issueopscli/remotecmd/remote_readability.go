package remotecmd

import (
	"issueops/internal/domain/artifactreadability"
	port "issueops/internal/port"
)

// The create responses carry the provider result unchanged and add the
// readability report of the body that was (or would be) published. The port
// types stay as they are; only the CLI JSON gains the readability key.
type createIssueResponse struct {
	port.IssueProviderCreateIssueResult
	Readability artifactreadability.Report `json:"readability"`
}

type createChildResponse struct {
	port.IssueProviderCreateChildResult
	Readability artifactreadability.Report `json:"readability"`
}

type createPRResponse struct {
	port.IssueProviderCreatePullRequestResult
	Readability artifactreadability.Report `json:"readability"`
}

type reflectCompletionResponse struct {
	port.IssueProviderUpdateIssueBodySectionResult
	Readability artifactreadability.Report `json:"readability"`
}

// readabilityDeferredCodes are the template findings the readability report
// already restates (summary-first, required sections, placeholders, Korean).
// A preview shows them in the report instead of failing, and a confirm is
// refused through the report. Every other template finding is an input error
// that fails in both modes.
var readabilityDeferredCodes = map[string]bool{
	"summary_section_missing":  true,
	"required_section_missing": true,
	"placeholder_section":      true,
	"missing_required_fields":  true,
	"korean_body_required":     true,
}
