package issueopsremote

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type MergeProviderResolver func(string) (port.IssueProviderPullRequestMerger, error)

// RemoteMergeService merges the PR/MR of a completed cycle. It never writes the
// record: cleanup re-reads the merged state from the provider itself.
type RemoteMergeService struct {
	records IssueRecordReader
	resolve MergeProviderResolver
}

func NewRemoteMergeService(records IssueRecordReader, resolve MergeProviderResolver) *RemoteMergeService {
	return &RemoteMergeService{records: records, resolve: resolve}
}

// Merge previews without confirm. With confirm it refuses on any blocker,
// then marks a draft ready, merges pinned to the completed head, and requires
// the readback to show the merge.
func (s *RemoteMergeService) Merge(ctx context.Context, id, method string, confirm bool) (model.RemoteMergeResult, error) {
	method, err := domain.NormalizeMergeMethod(method)
	if err != nil {
		return model.RemoteMergeResult{}, err
	}
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteMergeResult{}, err
	}
	target, err := domain.ResolveMergeTarget(record)
	if err != nil {
		return model.RemoteMergeResult{}, err
	}
	merger, err := s.resolve(target.Provider)
	if err != nil {
		return model.RemoteMergeResult{}, err
	}
	before, err := merger.ReadPullRequestMergeState(ctx, record.Repo, target.URL)
	if err != nil {
		return model.RemoteMergeResult{}, err
	}
	result := projectMergeResult(record.ID, target, method, before)
	if before.State == "merged" {
		result.OK, result.Merged, result.AlreadyMerged = true, true, true
		result.NextCommand = cleanupStatusCommand(record.ID)
		return result, nil
	}
	if len(result.Blockers) > 0 {
		if confirm {
			return result, fmt.Errorf("merge-pr refused: %s", blockerCodes(result.Blockers))
		}
		return result, nil
	}
	if !confirm {
		result.OK = true
		result.NextCommand = fmt.Sprintf("issueops remote merge-pr --id %s --method %s --confirm --json", record.ID, method)
		return result, nil
	}
	after, err := merger.MergePullRequest(ctx, port.IssueProviderPullRequestMergeRequest{
		Repo: record.Repo, URL: target.URL, Method: method, HeadOID: target.ExpectedHead, MarkReady: before.Draft,
	})
	if err != nil {
		return result, err
	}
	result = projectMergeResult(record.ID, target, method, after)
	result.MarkedReady = before.Draft
	if after.State != "merged" {
		return result, fmt.Errorf("merge was not verified: state=%s", after.State)
	}
	result.OK, result.Merged = true, true
	result.NextCommand = cleanupStatusCommand(record.ID)
	return result, nil
}

func projectMergeResult(id string, target domain.MergeTarget, method string, state model.RemotePullRequestMergeState) model.RemoteMergeResult {
	return model.RemoteMergeResult{
		ID: id, Provider: target.Provider, URL: target.URL, Method: method,
		State: state.State, Draft: state.Draft, HeadOID: state.HeadOID, ExpectedHeadOID: target.ExpectedHead,
		BaseBranch: state.BaseBranch, Checks: state.Checks, FailingChecks: state.Failing, PendingChecks: state.Pending,
		Blockers: domain.MergeBlockers(state, target.ExpectedHead), MergeCommitOID: state.MergeCommitOID,
	}
}

func cleanupStatusCommand(id string) string {
	return fmt.Sprintf("issueops cleanup status --id %s --merged --json", id)
}

func blockerCodes(blockers []model.RemoteMergeBlocker) string {
	codes := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		codes = append(codes, blocker.Code)
	}
	return strings.Join(codes, ", ")
}
