// Package remoteverification defines the DTOs for remote provider readbacks
// used to verify issues, PRs/MRs, and child tasks. It sits in the contract layer.
package remoteverification

// Artifact is one provider readback. Merge, head and base must remain from the
// same observation so cleanup can compare the branch OID without a second read.
type Artifact struct {
	URL                                  string
	Labels, Assignees                    []string
	Merged                               bool
	HeadRefName, HeadRefOID, BaseRefName string
}

// Target identifies a remote issue or PR/MR by provider, kind, and URL.
type Target struct{ Provider, Kind, URL string }

// ChildTarget identifies a provider-native child task, with the host, project,
// and IID needed to address it.
type ChildTarget struct{ Provider, URL, Hostname, Project, Kind, IID string }

// TaskMetadata is the type information read back from a child task. Empty
// means the provider returned no metadata at all.
type TaskMetadata struct {
	Empty           bool
	Type, IssueType string
}
