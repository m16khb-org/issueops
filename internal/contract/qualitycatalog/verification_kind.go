package qualitycatalog

// VerificationKind classifies how a candidate's change is verified externally,
// making the tool-grounded vs documentary distinction EXPLICIT instead of
// guessing it from free-text VerifyWith strings (which cannot reliably tell a
// tool signal from a doc artifact from model self-critique).
type VerificationKind string

const (
	// ToolSignalKind: a code/correctness change whose VerifyWith MUST name an
	// executable external signal (test/build/lint/contract/smoke/coverage run or
	// a CLI command) — never model self-critique.
	ToolSignalKind VerificationKind = "tool_signal"
	// DocArtifactKind: a documentation/governance change whose verification is a
	// concrete produced artifact (ADR entry, README section, checklist, matrix,
	// transcript). Explicitly EXEMPT from the executable-signal rule and labeled
	// as such, so it is not falsely claimed to be tool-gated.
	DocArtifactKind VerificationKind = "doc_artifact"
)
