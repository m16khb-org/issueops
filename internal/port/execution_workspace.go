package port

import (
	"context"
	executionissue "issueops/internal/contract/executionissue"
)

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

type ExecutionWorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Exists         bool   `json:"exists,omitempty"`
}

type ExecutionWorkspaceProvisioner interface {
	Prepare(context.Context, ExecutionWorkspaceRequest) (ExecutionWorkspaceReceipt, error)
}

type ExecutionWorkspaceAccessResult struct {
	Allowed         bool   `json:"allowed"`
	Code            string `json:"code,omitempty"`
	RelaunchCommand string `json:"relaunch_command,omitempty"`
}

type ExecutionWorkspaceAccessProber interface {
	ProbeAccess(context.Context, ExecutionWorkspaceRequest, string) (ExecutionWorkspaceAccessResult, error)
}

type ExecutionIssueSnapshotReader interface {
	ReadIssueSnapshot(context.Context, executionissue.ExecutionIssueSnapshotRequest) (executionissue.ExecutionIssueSnapshot, error)
}
