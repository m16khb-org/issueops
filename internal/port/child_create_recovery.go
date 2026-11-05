// Package port declares the interfaces and shared request and result types through which the
// application layer reaches adapters: issue providers, transactional record stores, workspace
// provisioning, native install hosts, MCP service control, and cleanup and replacement records. It
// contains no behavior; adapters implement these contracts and the composition root wires them.
package port

import "context"

// ChildSnapshot is the live state of a provider-native child issue read back during child-create
// recovery. TypeVerified reports that the item is an issue rather than a pull request, and
// HierarchyVerified that it is listed under the expected parent.
type ChildSnapshot struct {
	URL               string
	Title             string
	Body              string
	TypeVerified      bool
	HierarchyVerified bool
	Labels, Assignees []string
}

// IssueProviderChildCreateRecovery lets a provider recover an interrupted child create: find
// candidates carrying the create marker, read a child back given repo, parent URL and child URL,
// and attach it to its parent.
type IssueProviderChildCreateRecovery interface {
	FindChildCreateCandidates(context.Context, IssueProviderFindIssueCreateCandidatesRequest) (IssueProviderFindIssueCreateCandidatesResult, error)
	ReadChild(context.Context, string, string, string) (ChildSnapshot, error)
	AttachChild(context.Context, string, string, string) error
}
