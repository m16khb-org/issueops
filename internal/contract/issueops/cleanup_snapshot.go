package issueops

// CleanupSnapshot carries the record and its exact persistence revision.
// Revision is opaque to application code and never sent to a provider.
type CleanupSnapshot struct {
	Record   IssueOpsRecord
	Revision string
}
