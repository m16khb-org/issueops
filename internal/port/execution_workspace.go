package port

import (
	"context"
	executionissue "issueops/internal/contract/executionissue"
)

// ExecutionWorkspaceRequest describes the execution worktree to provision: its lifecycle, source
// repository, target root and branch, the base branch and head it starts from, an optional parent
// worktree, and whether the change is confirmed or a dry run.
type ExecutionWorkspaceRequest struct {
	LifecycleID    string `json:"lifecycle_id"`
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseBranch     string `json:"base_branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Confirm        bool   `json:"confirm,omitempty"`
}

// ExecutionWorkspaceReceipt reports the worktree a provisioner prepared. Exists is set when the
// worktree was already present.
type ExecutionWorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Exists         bool   `json:"exists,omitempty"`
}

// ExecutionWorkspaceProvisioner prepares the git worktree that an execution lease works in.
type ExecutionWorkspaceProvisioner interface {
	Prepare(context.Context, ExecutionWorkspaceRequest) (ExecutionWorkspaceReceipt, error)
}

// ExecutionWorkspaceAccessResult is the outcome of probing whether a host session may use a
// workspace; when access is denied, Code names the reason and RelaunchCommand how to restart the
// host in the workspace.
type ExecutionWorkspaceAccessResult struct {
	Allowed         bool   `json:"allowed"`
	Code            string `json:"code,omitempty"`
	RelaunchCommand string `json:"relaunch_command,omitempty"`
}

// ExecutionWorkspaceAccessProber checks, for a named host, whether the requested workspace is
// accessible without provisioning it. Provisioners may implement it optionally.
type ExecutionWorkspaceAccessProber interface {
	ProbeAccess(context.Context, ExecutionWorkspaceRequest, string) (ExecutionWorkspaceAccessResult, error)
}

// ExecutionIssueSnapshotReader reads the provider issue snapshot an execution is based on.
type ExecutionIssueSnapshotReader interface {
	ReadIssueSnapshot(context.Context, executionissue.SnapshotRequest) (executionissue.Snapshot, error)
}
