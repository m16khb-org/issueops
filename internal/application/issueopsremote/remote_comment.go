package issueopsremote

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remotedomain "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
)

type CommentProviderResolver func(string) (port.IssueProviderCommenter, error)

// RemoteCommentService posts one user-requested comment on the cycle's issue
// or PR/MR. QA: asked to leave a progress comment, an agent found no issueops
// path and stopped to ask for a raw `gh issue comment`, which skips the
// Korean, length and secret checks every other remote write gets.
type RemoteCommentService struct {
	records IssueRecordReader
	resolve CommentProviderResolver
}

func NewRemoteCommentService(records IssueRecordReader, resolve CommentProviderResolver) *RemoteCommentService {
	return &RemoteCommentService{records: records, resolve: resolve}
}

// Comment previews without confirm. A failed post is not retried: whether the
// provider stored the comment is unknown, and a second post duplicates it.
func (s *RemoteCommentService) Comment(ctx context.Context, id, on, body string, confirm bool) (model.RemoteCommentResult, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteCommentResult{}, err
	}
	target, err := commentTarget(record, on)
	if err != nil {
		return model.RemoteCommentResult{}, err
	}
	provider := domain.ResolveRecordProvider(record)
	if provider == "" {
		return model.RemoteCommentResult{}, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	result := model.RemoteCommentResult{ID: record.ID, Provider: provider, On: on, TargetURL: target, BodyChars: utf8.RuneCountInString(strings.TrimSpace(body))}
	if err := remotedomain.ValidateComment(provider, body); err != nil {
		return result, err
	}
	if !confirm {
		result.OK = true
		return result, nil
	}
	commenter, err := s.resolve(provider)
	if err != nil {
		return result, err
	}
	url, err := commenter.Comment(ctx, record.Repo, target, on, strings.TrimSpace(body))
	if err != nil {
		return result, fmt.Errorf("%w; the comment may have been posted: read the %s's comments before posting again", err, on)
	}
	result.OK, result.Posted, result.CommentURL = true, true, url
	return result, nil
}

func commentTarget(record model.IssueOpsRecord, on string) (string, error) {
	switch on {
	case "issue":
		if url := strings.TrimSpace(record.IssueURL); url != "" {
			return url, nil
		}
		return "", fmt.Errorf("the cycle has no linked issue yet")
	case "pr":
		if record.RemoteArtifact != nil && strings.TrimSpace(record.RemoteArtifact.URL) != "" {
			return strings.TrimSpace(record.RemoteArtifact.URL), nil
		}
		return "", fmt.Errorf("the cycle has no verified PR/MR yet (remote create-pr, then remote verify-artifact)")
	}
	return "", fmt.Errorf("--on must be issue or pr (a GitLab MR is pr), got %q", on)
}
