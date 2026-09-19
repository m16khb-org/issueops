package issueopsapp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"issueops/cmd/issueops/issueopscli/executioncmd"
	issueopsnextinbound "issueops/internal/adapter/inbound/issueopsnext"
	issueopscore "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/implementation"
	issueopsinventoryoutbound "issueops/internal/adapter/outbound/issueopsinventory"
	"issueops/internal/adapter/outbound/issueopsrecord"
	preflightadapter "issueops/internal/adapter/preflight"
	issueopsnextapplication "issueops/internal/application/issueopsnext"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	"issueops/internal/port"
)

// issueOpsNextHandler는 단계 분류에 필요한 관측을 꽂는다. 전부 읽기 전용이며,
// fetch도 provider 호출도 Orca 호출도 없다.
func issueOpsNextHandler(
	stagedArtifactNames func(stateRoot, id string) ([]string, error),
	observers ...issueopsrecord.Observer,
) func(
	string,
	string,
	string,
) (issueopsnextcontract.Result, error) {
	listCycles := issueOpsInventoryListHandler(observers...)
	return func(stateRoot, cwd, id string) (issueopsnextcontract.Result, error) {
		localObservation := nextLocalReadinessObservation{
			observe:  issueopscore.ObserveIssueOpsLocalPRReadiness,
			fallback: implementation.ObservedChangedPaths,
		}
		service := issueopsnextapplication.NewService(issueopsnextapplication.Ports{
			ListCycles: func(ctx context.Context, stateRoot, repo string) (issueopsinventorycontract.ListResult, error) {
				return listCycles(stateRoot, repo)
			},
			ReadRecord:          issueopscore.ReadIssueOps,
			Completion:          issueopscore.IssueOpsPhaseCompletion,
			LocalReadiness:      localObservation.localReadiness,
			WriterlessCommand:   issueopscore.ExecutionWriterAbsentRecoveryCommand,
			PlannerDefaults:     port.IssueOpsPlannerDefaults,
			ChangedPaths:        localObservation.changedPaths,
			ReviewEffortForTier: port.IssueOpsReviewEffortForTier,
			StagedArtifacts:     stagedArtifactNames,
			Actor: func() (string, string, error) {
				host, sessionID, _, err := executioncmd.ResolveNativeSessionIdentity(os.Getenv)
				return host, sessionID, err
			},
			ProcessLive:   observeIssueOpsHolderLiveness,
			SourceRoot:    issueopsinventoryoutbound.CleanPath{}.Normalize,
			CleanPath:     filepath.Clean,
			WorktreeState: observeIssueOpsWorktreeState,
			CurrentBranch: func(cwd string) string {
				return strings.TrimSpace(preflightadapter.GitOut(cwd, "branch", "--show-current"))
			},
			Env: os.Getenv,
			Now: time.Now,
		})
		return issueopsnextinbound.NewNextHandler(service)(stateRoot, cwd, id)
	}
}

type nextLocalReadinessObservation struct {
	observe  func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsReadiness, implementation.LocalChangeObservation)
	fallback func(issueopscontract.IssueOpsRecord) ([]string, bool)
	record   nextObservationRecord
	changes  implementation.LocalChangeObservation
	set      bool
}

type nextObservationRecord struct {
	id, repo, worktree, executionRoot string
	branch, baseBranch, baseSHA       string
}

func (observation *nextLocalReadinessObservation) localReadiness(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	ready, changes := observation.observe(record)
	observation.record = nextObservationRecordOf(record)
	observation.changes = changes
	observation.set = true
	return ready
}

func (observation *nextLocalReadinessObservation) changedPaths(record issueopscontract.IssueOpsRecord) ([]string, bool) {
	if observation.set && observation.record == nextObservationRecordOf(record) {
		return append([]string{}, observation.changes.Paths...), observation.changes.Verified
	}
	if observation.fallback == nil {
		return nil, false
	}
	return observation.fallback(record)
}

func nextObservationRecordOf(record issueopscontract.IssueOpsRecord) nextObservationRecord {
	key := nextObservationRecord{
		id: record.ID, repo: record.Repo, worktree: record.WorktreePath, branch: record.Branch,
	}
	if record.Execution != nil {
		key.executionRoot = record.Execution.Workspace.Root
	}
	if record.BranchPrepare != nil {
		key.baseBranch = record.BranchPrepare.BaseBranch
		key.baseSHA = record.BranchPrepare.BaseSHA
	}
	return key
}

// observeIssueOpsHolderLiveness는 PID 재사용까지 판정하는 기존 관측을 그대로
// 쓴다. 관측하지 못하면 nil을 돌려주고, 분류기가 그것을 살아 있는 홀더로
// 취급한다 — 확인하지 않은 세션의 lease를 빼앗으라고 권하지 않기 위해서다.
func observeIssueOpsHolderLiveness(receipt issueopscontract.NativeProcessReceipt) *bool {
	status, _, err := issueopscore.InspectNativeProcessReceipt(receipt)
	if err != nil {
		return nil
	}
	switch status {
	case issueopscore.NativeProcessStatusLive:
		live := true
		return &live
	case issueopscore.NativeProcessStatusDead, issueopscore.NativeProcessStatusIdentityMismatch:
		live := false
		return &live
	default:
		return nil
	}
}

func observeIssueOpsWorktreeState(root string) (bool, string, string) {
	if strings.TrimSpace(root) == "" {
		return false, "", ""
	}
	code, out, _ := preflightadapter.GitCmd(root, "rev-parse", "--show-toplevel")
	if code != 0 || strings.TrimSpace(out) == "" {
		return false, "", ""
	}
	return true,
		strings.TrimSpace(preflightadapter.GitOut(root, "branch", "--show-current")),
		strings.TrimSpace(preflightadapter.GitOut(root, "rev-parse", "HEAD"))
}
