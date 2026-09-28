package issueops

// CleanupFinishSnapshot carries the record and its exact persistence revision.
// Revision is opaque to application code and never sent to a provider.
type CleanupFinishSnapshot struct {
	Record   IssueOpsRecord
	Revision string
}
