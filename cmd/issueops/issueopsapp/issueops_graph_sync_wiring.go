package issueopsapp

import (
	"context"

	core "issueops/internal/adapter/issueops"
	application "issueops/internal/application/issueopsremote"
)

func newIssueGraphSyncService(root string) *application.IssueGraphSyncService {
	return application.NewIssueGraphSyncService(core.RemoteRecordStore{StateRoot: root}, core.IssueGraphPoster{})
}

func syncIssueGraph(ctx context.Context, root, id string, confirm bool) (map[string]any, error) {
	return newIssueGraphSyncService(root).Sync(ctx, id, confirm)
}
