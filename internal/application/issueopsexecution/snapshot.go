package issueopsexecution

import (
	"context"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"issueops/internal/contract/issueops"
	snapshotdomain "issueops/internal/domain/issueops"
	"strings"
)

func (s Service) snapshotReader(
	stateRoot string,
	req issueops.ExecutionActionRequest,
	fallback executionissue.ExecutionIssueSnapshotReadFunc,
) (executionissue.ExecutionIssueSnapshotReadFunc, func() string, error) {
	if req.IssueSnapshot == nil {
		return executionGitLabFallbackSnapshotReader(fallback)
	}
	if err := snapshotdomain.ValidateExecutionSnapshotAction(req); err != nil {
		return nil, nil, err
	}
	record, err := s.ReadRecord(stateRoot, req.ID)
	if err != nil {
		return nil, nil, err
	}
	err = snapshotdomain.ValidateExecutionSnapshotEvidence(req, record)
	if err != nil {
		return nil, nil, err
	}
	evidence := *req.IssueSnapshot
	snapshot := executionissue.ExecutionIssueSnapshot{URL: evidence.WebURL, Body: evidence.Body, State: evidence.State, Source: evidence.Source}
	linkedURL := strings.TrimSpace(record.IssueURL)
	reader := func(ctx context.Context, provider string, snapshotReq executionissue.ExecutionIssueSnapshotRequest) (executionissue.ExecutionIssueSnapshot, error) {
		if err := ctx.Err(); err != nil {
			return executionissue.ExecutionIssueSnapshot{}, err
		}
		if provider != "gitlab" || !s.SamePath(snapshotReq.Repo, record.Repo) || strings.TrimSpace(snapshotReq.URL) != linkedURL {
			return executionissue.ExecutionIssueSnapshot{}, fmt.Errorf("issue_snapshot request does not match the linked IssueOps identity")
		}
		snapshot.URL = linkedURL
		return snapshot, nil
	}
	return reader, func() string { return "glab_mcp" }, nil
}

func executionGitLabFallbackSnapshotReader(
	fallback executionissue.ExecutionIssueSnapshotReadFunc,
) (executionissue.ExecutionIssueSnapshotReadFunc, func() string, error) {
	source := ""
	reader := func(ctx context.Context, provider string, req executionissue.ExecutionIssueSnapshotRequest) (executionissue.ExecutionIssueSnapshot, error) {
		if fallback == nil {
			err := fmt.Errorf("remote issue snapshot reader is unavailable")
			if provider == "gitlab" {
				return executionissue.ExecutionIssueSnapshot{}, fmt.Errorf("gitlab_issue_snapshot_unavailable: %w", err)
			}
			return executionissue.ExecutionIssueSnapshot{}, err
		}
		snapshot, err := fallback(ctx, provider, req)
		if err != nil {
			if provider == "gitlab" {
				return executionissue.ExecutionIssueSnapshot{}, fmt.Errorf("gitlab_issue_snapshot_unavailable: %w", err)
			}
			return executionissue.ExecutionIssueSnapshot{}, err
		}
		if provider != "gitlab" {
			return snapshot, nil
		}
		if err := snapshotdomain.ValidateGitLabExecutionSnapshot(req.URL, snapshot.URL, snapshot.Body, snapshot.State); err != nil {
			return executionissue.ExecutionIssueSnapshot{}, fmt.Errorf("gitlab_issue_snapshot_unavailable: %w", err)
		}
		snapshot.URL = strings.TrimSpace(req.URL)
		snapshot.Source = "glab_cli"
		source = snapshot.Source
		return snapshot, nil
	}
	return reader, func() string { return source }, nil
}

func withExecutionIssueSnapshotSource(result any, source string) any {
	if source == "" {
		return result
	}
	switch typed := result.(type) {
	case issueops.ExecutionPrepareResult:
		typed.IssueSnapshotSource = source
		return typed
	case issueops.ExecutionResult:
		typed.IssueSnapshotSource = source
		return typed
	case issueops.ExecutionReplaceResult:
		typed.IssueSnapshotSource = source
		return typed
	case issueops.ExecutionReconcileResult:
		typed.IssueSnapshotSource = source
		return typed
	default:
		return result
	}
}
