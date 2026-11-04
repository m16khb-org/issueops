package issueopsremote

import (
	"context"

	domain "issueops/internal/domain/issueops"
)

type issueGraphPoster interface {
	Post(ctx context.Context, provider, issueURL, body string) (string, error)
}

// IssueGraphSyncService 타입은 기록의 이슈 그래프 링크를 provider의 이슈에 댓글로 동기화한다.
type IssueGraphSyncService struct {
	records issueRecordReader
	poster  issueGraphPoster
}

// NewIssueGraphSyncService 함수는 기록 조회와 댓글 게시기를 묶어 서비스를 만든다.
func NewIssueGraphSyncService(records issueRecordReader, poster issueGraphPoster) *IssueGraphSyncService {
	return &IssueGraphSyncService{records: records, poster: poster}
}

// Sync 메서드는 동기화 계획을 세워 미리보기/변경 없음이면 그 결과를, 아니면 댓글을 게시하고 URL을 담은 결과를 돌려준다.
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
	url, err := s.poster.Post(ctx, plan.Provider, plan.URL, plan.Body)
	if err != nil {
		return nil, err
	}
	key := "comment_url"
	if plan.Provider == "gitlab" {
		key = "note_url"
	}
	return map[string]any{"ok": true, "synced": true, "provider": plan.Provider, key: url, "link_count": plan.LinkCount}, nil
}
