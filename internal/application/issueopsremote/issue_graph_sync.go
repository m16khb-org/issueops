package issueopsremote

import (
	"context"

	domain "issueops/internal/domain/issueops"
)

type IssueGraphPoster interface {
	Post(provider, issueURL, body string) (string, error)
}

type IssueGraphSyncService struct {
	records IssueRecordReader
	poster  IssueGraphPoster
}

func NewIssueGraphSyncService(records IssueRecordReader, poster IssueGraphPoster) *IssueGraphSyncService {
	return &IssueGraphSyncService{records: records, poster: poster}
}

func (s *IssueGraphSyncService) Sync(ctx context.Context, id string, confirm bool) (map[string]any, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return nil, err
	}
	plan, err := domain.PlanIssueGraphSync(record, confirm)
	if err != nil {
		return nil, err
	}
	if plan.Preview {
		return map[string]any{"ok": true, "synced": false, "dry_run": true, "link_count": plan.LinkCount, "message": plan.Message}, nil
	}
	if plan.Noop {
		return map[string]any{"ok": true, "synced": false, "message": plan.Message}, nil
	}
	url, err := s.poster.Post(plan.Provider, plan.URL, plan.Body)
	if err != nil {
		return nil, err
	}
	key := "comment_url"
	if plan.Provider == "gitlab" {
		key = "note_url"
	}
	return map[string]any{"ok": true, "synced": true, "provider": plan.Provider, key: url, "link_count": plan.LinkCount}, nil
}
