package issueopsremote

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/secretdetection"
	"issueops/internal/port"
)

type MilestoneProviderResolver func(string) (port.IssueProviderMilestoneSetter, error)

// RemoteMilestoneService puts the cycle's issue or PR/MR on an existing
// milestone on request. QA: asked to file an issue under milestone v1.1, an
// agent ran raw `gh issue edit --milestone`. The milestone must already exist;
// the provider refuses an unknown title and nothing is created.
type RemoteMilestoneService struct {
	records IssueRecordReader
	resolve MilestoneProviderResolver
}

func NewRemoteMilestoneService(records IssueRecordReader, resolve MilestoneProviderResolver) *RemoteMilestoneService {
	return &RemoteMilestoneService{records: records, resolve: resolve}
}

func (s *RemoteMilestoneService) SetMilestone(ctx context.Context, id, on, title string, confirm bool) (model.RemoteMilestoneResult, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteMilestoneResult{}, err
	}
	target, err := commentTarget(record, on)
	if err != nil {
		return model.RemoteMilestoneResult{}, err
	}
	provider := domain.ResolveRecordProvider(record)
	if provider == "" {
		return model.RemoteMilestoneResult{}, fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
	}
	title = strings.TrimSpace(title)
	result := model.RemoteMilestoneResult{ID: record.ID, Provider: provider, On: on, TargetURL: target, Milestone: title}
	if title == "" {
		return result, fmt.Errorf("--set MILESTONE_TITLE is required")
	}
	if strings.ContainsAny(title, "\r\n") {
		return result, fmt.Errorf("--set must be one line")
	}
	if match, found := secretdetection.Find(title); found {
		return result, fmt.Errorf("--set contains a secret-like value (%s); remove it", match.Kind)
	}
	if !confirm {
		result.OK = true
		return result, nil
	}
	setter, err := s.resolve(provider)
	if err != nil {
		return result, err
	}
	readback, err := setter.SetMilestone(ctx, record.Repo, target, on, title)
	if err != nil {
		return result, fmt.Errorf("%w; the milestone must already exist in the repository (it is never created)", err)
	}
	result.ReadBack = readback
	if strings.TrimSpace(readback) != title {
		return result, fmt.Errorf("the %s milestone reads back %q, not %q", on, readback, title)
	}
	result.OK, result.Set = true, true
	return result, nil
}
