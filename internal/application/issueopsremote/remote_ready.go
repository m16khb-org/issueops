package issueopsremote

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// MarkReady makes a completed cycle's draft PR/MR ready for review without
// merging it. QA: asked to "make the PR ready for review, do not merge", an
// agent had no issueops path (draft is cleared only inside merge-pr) and ran
// raw `gh pr ready`. Preview without confirm; with confirm the readback must
// show the draft cleared.
func (s *RemoteMergeService) MarkReady(ctx context.Context, id string, confirm bool) (model.RemoteReadyResult, error) {
	record, err := s.records.Read(ctx, id)
	if err != nil {
		return model.RemoteReadyResult{}, err
	}
	target, err := domain.ResolveReadyTarget(record)
	if err != nil {
		return model.RemoteReadyResult{}, err
	}
	merger, err := s.resolve(target.Provider)
	if err != nil {
		return model.RemoteReadyResult{}, err
	}
	before, err := merger.ReadPullRequestMergeState(ctx, record.Repo, target.URL)
	if err != nil {
		return model.RemoteReadyResult{}, err
	}
	result := projectReadyResult(record.ID, target, before)
	if before.State != "open" {
		return result, fmt.Errorf("mark-ready needs an open PR/MR; %s is %s", target.URL, before.State)
	}
	if !before.Draft {
		result.OK, result.AlreadyReady = true, true
		return result, nil
	}
	if head := strings.TrimSpace(before.HeadOID); head != target.ExpectedHead {
		return result, fmt.Errorf("mark-ready refused: the PR/MR head %s is not the completion head %s, so reviewers would see commits the completion did not verify; reseed the cycle to verify them first", head, target.ExpectedHead)
	}
	preview := fmt.Sprintf("issueops remote mark-ready --id %s --json", record.ID)
	if !confirm {
		result.OK = true
		result.NextCommand = fmt.Sprintf("issueops remote mark-ready --id %s --confirm --json", record.ID)
		return result, nil
	}
	readier, ok := merger.(port.IssueProviderPullRequestReadier)
	if !ok {
		return result, fmt.Errorf("provider %q cannot mark a pull request ready", target.Provider)
	}
	if err := readier.MarkPullRequestReady(ctx, record.Repo, target.URL); err != nil {
		result.NextCommand = preview
		return result, fmt.Errorf("%w; read the state with the preview (%s) before retrying", err, preview)
	}
	after, err := merger.ReadPullRequestMergeState(ctx, record.Repo, target.URL)
	if err != nil {
		result.NextCommand = preview
		return result, fmt.Errorf("draft state not read back: %w; run the preview (%s)", err, preview)
	}
	result = projectReadyResult(record.ID, target, after)
	if after.Draft {
		return result, fmt.Errorf("mark-ready was not verified: the PR/MR still reads back as a draft")
	}
	result.OK, result.MarkedReady = true, true
	return result, nil
}

func projectReadyResult(id string, target domain.MergeTarget, state model.RemotePullRequestMergeState) model.RemoteReadyResult {
	return model.RemoteReadyResult{
		ID: id, Provider: target.Provider, URL: target.URL, State: state.State, Draft: state.Draft,
		HeadOID: state.HeadOID, ExpectedHeadOID: target.ExpectedHead,
	}
}
