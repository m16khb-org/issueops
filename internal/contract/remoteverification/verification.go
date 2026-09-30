package remoteverification

// Artifact is one provider readback. Merge, head and base must remain from the
// same observation so cleanup can compare the branch OID without a second read.
type Artifact struct {
	URL                                  string
	Labels, Assignees                    []string
	Merged                               bool
	HeadRefName, HeadRefOID, BaseRefName string
}
type Target struct{ Provider, Kind, URL string }
type ChildTarget struct{ Provider, URL, Hostname, Project, Kind, IID string }
type TaskMetadata struct {
	Empty           bool
	Type, IssueType string
}
