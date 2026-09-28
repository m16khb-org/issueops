package issueops

// CompletionArtifactDigest names one staged artifact and its content digest
// for durable preservation in the issue body.
type CompletionArtifactDigest struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
}

// RemoteCompletionSection carries the completion payload rendered into
// the managed completion section. Blocks with empty values still render with a
// placeholder so the section shape stays machine-checkable.
type RemoteCompletionSection struct {
	FinalHead           string                     `json:"final_head"`
	RemoteArtifactURL   string                     `json:"remote_artifact_url"`
	VerificationSummary []string                   `json:"verification_summary"`
	ArtifactManifest    []CompletionArtifactDigest `json:"artifact_manifest"`
	TuringSummary       string                     `json:"turing_summary"`
	SpecBody            string                     `json:"spec_body"`
	PlanBody            string                     `json:"plan_body"`
	CleanupAudit        string                     `json:"cleanup_audit,omitempty"`
	// MissingArtifacts lists required sealed artifacts (plan) that were absent
	// from the workspace artifact directory when the section was gathered (#482).
	MissingArtifacts []string `json:"missing_artifacts,omitempty"`
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
