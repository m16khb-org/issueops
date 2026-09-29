package issueops

import (
	preflightadapter "issueops/internal/adapter/preflight"
	model "issueops/internal/contract/issueops"
)

var GitOut = preflightadapter.GitOut

var GitCmd = preflightadapter.GitCmd
var GitCmdRaw = preflightadapter.GitCmdRaw

func workspaceSnapshot(workspace model.Workspace) (string, error) {
	return (LeaseWorkspaceSnapshot{GitCmd: GitCmd, GitCmdRaw: GitCmdRaw}).Snapshot(workspace)
}
