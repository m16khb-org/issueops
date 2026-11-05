package issueopscleanup

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type statusEnvironment interface {
	DirectoryExists(string) bool
	Git(string, ...string) (int, string, string)
	GitOutput(string, ...string) string
}

// StructuralStatus derives cleanup status from git facts observed in the
// record's worktree.
type StructuralStatus struct{ Environment statusEnvironment }

// ForRecord observes the record's worktree, current branch and first remote
// and returns the domain-built cleanup status. A missing worktree yields a
// status without git facts.
func (s StructuralStatus) ForRecord(record model.Record, req model.CleanupStatusRequest) model.CleanupStatus {
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

type statusRecords interface {
	Load(string) (model.Record, error)
}

// StatusService answers `cleanup status`, using the structural status unless
// merge readback is needed, in which case it reuses the finish preview.
type StatusService struct {
	Records          statusRecords
	Structural       StructuralStatus
	Provider         func(string) (port.IssueProvider, error)
	CurrentDirectory func() (string, error)
	PreviewFinish    func(context.Context, model.CleanupFinishRequest, port.IssueProvider) (model.CleanupFinishResult, error)
}

// Status returns the cleanup status for id. When merge readback is needed it
// resolves the provider and current directory and merges in the finish preview
// result; errors are returned only when the preview gave no usable gates.
func (s StatusService) Status(ctx context.Context, id string, mergedRequested bool) (model.CleanupStatus, error) {
	record, err := s.Records.Load(id)
	failed := model.CleanupStatus{OK: false, ID: id}
	if err != nil {
		return failed, err
	}
	if !domain.CleanupStatusNeedsMergeReadback(record, mergedRequested) {
		return s.Structural.ForRecord(record, model.CleanupStatusRequest{}), nil
	}
	// The finish preview observes the worktree and remote refs itself and its
	// result replaces the structural status, which then contributes only the
	// record's artifact URL: skip the git and network ls-remote observation.
	structural := domain.BuildCleanupStatus(record, model.CleanupStatusRequest{}, domain.CleanupStatusObservation{})
	providerName := domain.ResolveRecordProvider(record)
	if providerName == "" {
		return failed, fmt.Errorf("cannot determine provider from IssueOps record")
	}
	prov, err := s.Provider(providerName)
	if err != nil {
		return failed, err
	}
	cwd, err := s.CurrentDirectory()
	if err != nil {
		return failed, fmt.Errorf("cannot resolve current directory (refusing cleanup status): %w", err)
	}
	req := model.CleanupFinishRequest{ID: record.ID, CWD: cwd}
	result, err := s.PreviewFinish(ctx, req, prov)
	if err != nil && (result.ID != id || len(result.Missing) == 0) {
		return failed, err
	}
	return domain.CleanupStatusFromFinish(structural, result), nil
}
