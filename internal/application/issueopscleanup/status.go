package issueopscleanup

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type StatusEnvironment interface {
	DirectoryExists(string) bool
	Git(string, ...string) (int, string, string)
	GitOutput(string, ...string) string
}

type StructuralStatus struct{ Environment StatusEnvironment }

func (s StructuralStatus) ForRecord(record model.IssueOpsRecord, req model.IssueOpsCleanupStatusRequest) model.IssueOpsCleanupStatus {
	var facts domain.CleanupStatusObservation
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" {
		return domain.BuildCleanupStatus(record, req, facts)
	}
	facts.WorktreeExists = s.Environment.DirectoryExists(worktree)
	if !facts.WorktreeExists {
		return domain.BuildCleanupStatus(record, req, facts)
	}
	facts.StatusCode, facts.StatusOutput, facts.StatusError = s.Environment.Git(worktree, "status", "--porcelain=v1")
	facts.Branch = strings.TrimSpace(s.Environment.GitOutput(worktree, "branch", "--show-current"))
	remotes := strings.Fields(s.Environment.GitOutput(worktree, "remote"))
	if len(remotes) > 0 {
		facts.Remote = remotes[0]
	}
	if facts.Remote != "" && facts.Branch != "" {
		facts.RemoteCode, facts.RemoteOutput, facts.RemoteError = s.Environment.Git(worktree, "ls-remote", "--heads", facts.Remote, facts.Branch)
	}
	return domain.BuildCleanupStatus(record, req, facts)
}

type StatusRecords interface {
	Load(string) (model.IssueOpsRecord, error)
}

type StatusService struct {
	Records           StatusRecords
	Structural        StructuralStatus
	Provider          func(string) (port.IssueProvider, error)
	VerifyMergedHead  func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error)
	ReadIssueSnapshot func(context.Context, port.IssueProvider, port.ExecutionIssueSnapshotRequest) (port.ExecutionIssueSnapshot, error)
	CurrentDirectory  func() (string, error)
	PreviewFinish     func(context.Context, model.CleanupFinishRequest, port.IssueProvider) (model.CleanupFinishResult, error)
}

func (s StatusService) Status(ctx context.Context, id string, mergedRequested bool) (model.IssueOpsCleanupStatus, error) {
	record, err := s.Records.Load(id)
	failed := model.IssueOpsCleanupStatus{OK: false, ID: id}
	if err != nil {
		return failed, err
	}
	structural := s.Structural.ForRecord(record, model.IssueOpsCleanupStatusRequest{})
	if !domain.CleanupStatusNeedsMergeReadback(record, mergedRequested) {
		return structural, nil
	}
	providerName := domain.ResolveRecordProvider(record)
	if providerName == "" {
		return failed, fmt.Errorf("cannot determine provider from IssueOps record")
	}
	prov, err := s.Provider(providerName)
	if err != nil {
		return failed, err
	}
	if s.VerifyMergedHead == nil {
		return failed, fmt.Errorf("merge verification is not configured")
	}
	mergedArtifact, err := s.VerifyMergedHead(*record.RemoteArtifact)
	if err != nil {
		return failed, fmt.Errorf("merge evidence readback failed (refusing to continue): %w", err)
	}
	snapshot, err := s.ReadIssueSnapshot(ctx, prov, port.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return failed, fmt.Errorf("issue readback failed (refusing to continue): %w", err)
	}
	cwd, err := s.CurrentDirectory()
	if err != nil {
		return failed, fmt.Errorf("cannot resolve current directory (refusing cleanup status): %w", err)
	}
	req := domain.WithCleanupFinishEvidence(model.CleanupFinishRequest{ID: record.ID, CWD: cwd, Merged: true}, snapshot.Body, snapshot.State, mergedArtifact)
	result, err := s.PreviewFinish(ctx, req, prov)
	if err != nil && (result.ID != id || len(result.Missing) == 0) {
		return failed, err
	}
	return domain.CleanupStatusFromFinish(structural, result), nil
}
