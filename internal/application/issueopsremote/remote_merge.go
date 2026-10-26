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

// RemoteMergeService merges the PR/MR of a completed cycle. Cleanup re-reads
// the merged state from the provider itself; the record only gets a merged
// receipt so next can route past the merge.
type RemoteMergeService struct {
	records IssueRecordReader
	resolve MergeProviderResolver
	merged  func(context.Context, string) (model.IssueOpsRecord, error)
}

// WithMergedReceipt records merged_at once the provider reads the PR/MR back
// merged. A failed receipt write never turns a done merge into an error.
func (s *RemoteMergeService) WithMergedReceipt(record func(context.Context, string) (model.IssueOpsRecord, error)) *RemoteMergeService {
	s.merged = record
	return s
}

func (s *RemoteMergeService) recordMerged(ctx context.Context, record model.IssueOpsRecord, result *model.RemoteMergeResult) {
	if s.merged == nil || (record.RemoteCompletion != nil && record.RemoteCompletion.MergedAt != "") {
		return
	}
	if _, err := s.merged(ctx, record.ID); err != nil {
		result.Warnings = append(result.Warnings, "merged_receipt_not_recorded: "+err.Error()+"; next may name merge-pr again, whose preview answers already_merged")
	}
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
		result.NextCommand = postMergeCommand(record.ID)
		s.recordMerged(ctx, record, &result)
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
		result.MarkReadyOnConfirm = before.Draft
		result.NextCommand = fmt.Sprintf("issueops remote merge-pr --id %s --method %s --confirm --json", record.ID, method)
		return result, nil
	}
	after, err := merger.MergePullRequest(ctx, port.IssueProviderPullRequestMergeRequest{
		Repo: record.Repo, URL: target.URL, Method: method, HeadOID: target.ExpectedHead, MarkReady: before.Draft,
	})
	if alternative, refused := domain.MergeMethodRefusedAlternative(err, method); refused {
		result.NextCommand = fmt.Sprintf("issueops remote merge-pr --id %s --method %s --json", record.ID, alternative)
		return result, fmt.Errorf("%w; the repository does not allow --method %s, so nothing was merged: preview another method the repository allows (merge, rebase, squash), e.g. %s", err, method, result.NextCommand)
	}
	if err != nil {
		// QA (lost merge response): the merge had happened while the CLI
		// reported a failure. The preview re-reads the provider and reports
		// already_merged, so it is the next step, never a second --confirm.
		result.NextCommand = fmt.Sprintf("issueops remote merge-pr --id %s --json", record.ID)
		return result, fmt.Errorf("%w; the merge may still have happened: read the state with the preview (issueops remote merge-pr --id %s --json) before anything else and do not repeat --confirm", err, record.ID)
	}
	result = projectMergeResult(record.ID, target, method, after)
	result.MarkedReady = before.Draft
	if after.State != "merged" {
		return result, fmt.Errorf("merge was not verified: state=%s", after.State)
	}
	result.OK, result.Merged = true, true
	result.NextCommand = postMergeCommand(record.ID)
	s.recordMerged(context.WithoutCancel(ctx), record, &result)
	return result, nil
}

func projectMergeResult(id string, target domain.MergeTarget, method string, state model.RemotePullRequestMergeState) model.RemoteMergeResult {
	return model.RemoteMergeResult{
		ID: id, Provider: target.Provider, URL: target.URL, Method: method,
		State: state.State, Draft: state.Draft, HeadOID: state.HeadOID, ExpectedHeadOID: target.ExpectedHead,
		BaseBranch: state.BaseBranch, Checks: state.Checks, FailingChecks: state.Failing, PendingChecks: state.Pending,
		Blockers: domain.MergeBlockers(state, target), MergeCommitOID: state.MergeCommitOID,
	}
}

// postMergeCommand is the step after a merge. The remote branch goes first
// (agents following cleanup status skipped it); its preview hands off to
// cleanup status, also when the branch is already gone.
func postMergeCommand(id string) string {
	return fmt.Sprintf("issueops cleanup remote-branch --id %s --preview --json", id)
}

func blockerCodes(blockers []model.RemoteMergeBlocker) string {
	codes := make([]string, 0, len(blockers))
	for _, blocker := range blockers {
		codes = append(codes, blocker.Code)
	}
	return strings.Join(codes, ", ")
}
