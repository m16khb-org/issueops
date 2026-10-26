package issueopsremote

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	remotedomain "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/secretdetection"
	"issueops/internal/port"
)

type RetitleProviderResolver func(string) (port.IssueProviderRetitler, error)

// RemoteRetitleService changes the title of the cycle's issue or PR/MR on
// request. QA: asked to rename the MR, an agent found no issueops path, ran
// `glab mr update --title` (exit 0, title unchanged) and then a raw API PUT.
type RemoteRetitleService struct {
	records IssueRecordReader
	resolve RetitleProviderResolver
}

func NewRemoteRetitleService(records IssueRecordReader, resolve RetitleProviderResolver) *RemoteRetitleService {
	return &RemoteRetitleService{records: records, resolve: resolve}
}

func (s *RemoteRetitleService) Retitle(ctx context.Context, id, on, title string, confirm bool) (model.RemoteRetitleResult, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteRetitleResult{}, err
	}
	target, err := commentTarget(record, on)
	if err != nil {
		return model.RemoteRetitleResult{}, err
	}
	provider := domain.ResolveRecordProvider(record)
	if provider == "" {
		return model.RemoteRetitleResult{}, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	title = strings.TrimSpace(title)
	result := model.RemoteRetitleResult{ID: record.ID, Provider: provider, On: on, TargetURL: target, Title: title}
	if title == "" {
		return result, fmt.Errorf("--title TEXT is required")
	}
	if strings.ContainsAny(title, "\r\n") {
		return result, fmt.Errorf("--title must be one line")
	}
	if err := remotedomain.ValidateProviderTitleLength(on, title); err != nil {
		return result, err
	}
	if match, found := secretdetection.Find(title); found {
		return result, fmt.Errorf("--title contains a secret-like value (%s); remove it", match.Kind)
	}
	if !confirm {
		result.OK = true
		return result, nil
	}
	retitler, err := s.resolve(provider)
	if err != nil {
		return result, err
	}
	readback, err := retitler.Retitle(ctx, record.Repo, target, on, title)
	if err != nil {
		return result, fmt.Errorf("%w; read the %s's current title before setting it again", err, on)
	}
	result.ReadBack = readback
	if strings.TrimSpace(readback) != title {
		return result, fmt.Errorf("the %s title reads back %q, not %q", on, readback, title)
	}
	result.OK, result.Retitled = true, true
	return result, nil
}
