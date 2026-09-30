package issueops

// RemoteCompletionSection carries the human-written progress report. Harness
// evidence remains in the record and tracked materials, outside the issue body.
type RemoteCompletionSection struct {
	RemoteArtifactURL string `json:"remote_artifact_url"`
	ResultBody        string `json:"result_body"`
}

// PlanReviewRound projects a review verdict and finding count for issue readers.
type PlanReviewRound struct {
	Verdict  string `json:"verdict"`
	Findings int    `json:"findings"`
}

// Managed issue-body section kinds shared by core callers and provider
// adapters. The set is intentionally closed (no open extension point).
const (
	IssueBodySectionDevilsAdvocate = "devils-advocate"
	IssueBodySectionCompletion     = "completion"

	// IssueBodyCompletionStartMarker is the durable delimiter cleanup finish
	// readback-checks before destructive local cleanup (설계 v5 WS3).
	IssueBodyCompletionStartMarker = "<!-- issueops:completion:start -->"
)
