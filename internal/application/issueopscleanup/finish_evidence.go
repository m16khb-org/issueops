package issueopscleanup

import (
	"context"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type FinishEvidenceReader struct {
	Provider          port.IssueProvider
	VerifyMergedHead  func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error)
	ReadIssueSnapshot func(context.Context, port.IssueProvider, executionissue.ExecutionIssueSnapshotRequest) (executionissue.ExecutionIssueSnapshot, error)
}

// Observe derives evidence from the executor's exact record snapshot. Arm will
// reject a record change made while these external observations were in flight.
func (s FinishEvidenceReader) Observe(ctx context.Context, record model.IssueOpsRecord, req model.CleanupFinishRequest) (model.CleanupFinishRequest, error) {
	if record.RemoteArtifact == nil {
		// Probe: finish on an unpublished cycle named no way out.
		return req, fmt.Errorf("cleanup finish requires a verified remote artifact (the cycle's merged PR/MR); a cycle without a published PR/MR ends with issueops cleanup abandon --id %s --reason TEXT --preview", record.ID)
	}
	if s.VerifyMergedHead == nil {
		return req, fmt.Errorf("merge verification is not configured")
	}
	merged, err := s.VerifyMergedHead(*record.RemoteArtifact)
	req.Merged = err == nil
	req.SupersededBy = strings.TrimSpace(req.SupersededBy)
	if err != nil && req.SupersededBy == "" {
		return req, fmt.Errorf("merge evidence readback failed (refusing to continue): %w", err)
	}
	if !req.Merged {
		replacement := *record.RemoteArtifact
		replacement.URL = req.SupersededBy
		merged, err = s.VerifyMergedHead(replacement)
		if err != nil {
			return req, fmt.Errorf("superseding merge evidence readback failed (refusing to continue): %w", err)
		}
	}
	snapshot, err := s.ReadIssueSnapshot(ctx, s.Provider, executionissue.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return req, fmt.Errorf("issue readback failed (refusing to continue): %w", err)
	}
	return domain.WithCleanupFinishEvidence(req, snapshot.Body, snapshot.State, merged), nil
}
