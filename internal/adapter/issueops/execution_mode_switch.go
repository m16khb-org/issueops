package issueops

import (
	"context"
	"fmt"
	"os"
	"strings"

	modeswitchapp "issueops/internal/application/issueopsmodeswitch"
	"issueops/internal/contract/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	modeswitchdomain "issueops/internal/domain/issueopsmodeswitch"
)

// switchModeInventory는 fingerprint 입력이 되는 현재 관측 상태다.
type switchModeInventory struct {
	ID              string `json:"id"`
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	CurrentMode     string `json:"current_mode"`
	RequestedMode   string `json:"requested_mode"`
	WorktreeRoot    string `json:"worktree_root"`
	WorktreePresent bool   `json:"worktree_present"`
	BranchOID       string `json:"branch_oid"`
	LeaseStatus     string `json:"lease_status"`
	LeaseGeneration uint64 `json:"lease_generation"`
	PendingID       string `json:"pending_operation_id"`
}

// SwitchExecutionMode는 게이트를 평가하고, apply에서 워크스페이스를 정리한 뒤
// execution record를 제거해 다음 prepare가 요청한 모드로 새로 준비하게 한다.
//
// record를 새 모드로 덮어쓰지 않고 제거하는 이유는 워크스페이스 때문이다. 새
// 모드의 워크스페이스는 그 모드의 provisioner가 만들어야 하고(driver가 다르다),
// prepare가 이미 그 일을 한다. 여기서 절반만 채운 record를 남기면 두 곳이 같은
// 상태를 만들게 되어 어긋난다.
func SwitchExecutionMode(ctx context.Context, stateRoot string, req ExecutionSwitchModeRequest, deps ExecutionSwitchModeDependencies) (ExecutionSwitchModeResult, error) {
	if deps.Git == nil {
		deps.Git = func(dir string, args ...string) (int, string) {
			code, stdout, stderr := GitCmd(dir, args...)
			if code != 0 && stderr != "" {
				return code, stderr
			}
			return code, stdout
		}
	}
	requested, err := modeswitchdomain.NormalizeMode(req.Mode)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: req.ID}, err
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return ExecutionSwitchModeResult{OK: false, ID: req.ID}, fmt.Errorf(
			"IssueOps execution is not prepared; run `issueops execution prepare --id %s --mode %s` instead", record.ID, requested)
	}

	result := ExecutionSwitchModeResult{
		OK: true, ID: record.ID, Preview: !req.Apply,
		CurrentMode: string(record.Execution.Mode), RequestedMode: requested,
		LeaseGeneration: record.Execution.Lease.Generation,
	}
	inventory, missing := switchModeGates(record, requested, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("execution switch-mode is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := hashJSON(inventory)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"issueops execution switch-mode --id %s --mode %s --apply --confirm --fingerprint %s --json",
			record.ID, requested, fingerprint)
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("execution switch-mode --apply requires --confirm")
	}
	// TOCTOU: preview 이후 lease·pending·워크스페이스가 바뀌었다면 그 preview는
	// 다른 상태를 승인한 것이다(cleanup abandon과 같은 계약).
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale switch-mode fingerprint; run the preview again and retry with the new value")
	}
	expectedSHA, err := hashJSON(record)
	if err != nil {
		return ExecutionSwitchModeResult{OK: false, ID: record.ID}, err
	}
	err = modeswitchapp.Apply(ctx, modeswitchapp.ApplyRequest{
		ID: record.ID, Repo: record.Repo, WorktreeRoot: inventory.WorktreeRoot,
		WorktreePresent: inventory.WorktreePresent, Branch: inventory.Branch,
		BranchPresent: inventory.BranchOID != "", ExpectedRecordSHA: expectedSHA,
	}, &switchModeEffects{stateRoot: stateRoot, deps: deps})
	if err != nil {
		result.OK = false
		return result, err
	}
	result.WorktreePresent = false
	result.BranchPresent = false
	result.SwitchedAt = executionNow(nil)
	result.NextAction = fmt.Sprintf("prepare IssueOps execution in %s mode", requested)
	return result, nil
}

// switchModeGates는 게이트 전부를 평가하고 missing을 나열한다(첫 실패에 멈추지
// 않는다 — 운영자가 한 번의 preview로 모든 결격 사유를 본다).
func switchModeGates(record issueops.IssueOpsRecord, requested string, deps ExecutionSwitchModeDependencies, result *ExecutionSwitchModeResult) (switchModeInventory, []string) {
	execution := record.Execution
	inventory := switchModeInventory{
		ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(execution.Workspace.Branch),
		CurrentMode: string(execution.Mode), RequestedMode: requested,
		WorktreeRoot:    strings.TrimSpace(execution.Workspace.Root),
		LeaseStatus:     string(execution.Lease.Status),
		LeaseGeneration: execution.Lease.Generation,
	}
	result.WorktreeRoot = inventory.WorktreeRoot
	result.Branch = inventory.Branch
	facts := modeswitchdomain.Facts{
		CurrentMode: inventory.CurrentMode, RequestedMode: requested,
		WriterPresent: leasedomain.LeaseHoldsWriter(inventory.LeaseStatus), PendingIntent: execution.Pending != nil,
		WorktreeClean: true, NoUnpushedCommits: true,
	}

	// Mode, writer, and pending-intent eligibility are decided together by the
	// domain after the adapter has finished observing worktree and Git state.
	if execution.Pending != nil {
		inventory.PendingID = strings.TrimSpace(execution.Pending.OperationID)
	}
	// ④ 잃을 작업이 없어야 한다. 워크트리가 없으면 지울 것도 없으므로 통과다.
	if inventory.WorktreeRoot != "" {
		if _, err := os.Stat(inventory.WorktreeRoot); err == nil {
			inventory.WorktreePresent = true
			facts.WorktreePresent = true
			result.WorktreePresent = true
			code, out := deps.Git(inventory.WorktreeRoot, "status", "--porcelain=v1")
			facts.WorktreeClean = code == 0 && strings.TrimSpace(out) == ""
			// 푸시되지 않은 커밋은 워크트리를 지우면 사라진다. upstream이 없으면
			// 비교할 대상이 없으므로 커밋 존재 자체를 잃을 작업으로 본다.
			if inventory.Branch != "" {
				if code, out := deps.Git(inventory.WorktreeRoot, "rev-list", "--count", "refs/remotes/origin/"+inventory.Branch+".."+"HEAD"); code == 0 {
					facts.NoUnpushedCommits = strings.TrimSpace(out) == "0"
				} else if code, out := deps.Git(inventory.WorktreeRoot, "rev-list", "--count", strings.TrimSpace(execution.Workspace.BaseHead)+".."+"HEAD"); code != 0 || strings.TrimSpace(out) != "0" {
					// upstream을 못 읽으면 base 대비로 판정한다. 둘 다 실패하면
					// 관측 불가이므로 fail-closed다.
					facts.NoUnpushedCommits = false
				}
			}
		}
	}
	if inventory.Branch != "" {
		if code, out := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch); code == 0 {
			inventory.BranchOID = strings.TrimSpace(out)
			result.BranchPresent = true
		}
	}
	// ⑤ orca로 가려면 정리가 끝난 뒤에도 브랜치 이름이 비어 있어야 한다. Orca는
	// 언제나 새 브랜치를 만들고 이름이 쓰이고 있으면 접미사를 붙이므로
	// (#149·#154), 여기서 통과시키면 전환은 성공하는데 바로 다음 prepare가
	// orca_branch_name_taken으로 막힌다 — 워크트리만 잃고 제자리다.
	//
	// prepare의 ensureOrcaBranchIsFree를 재사용하지 않는다. 그 함수는 "지금 이름이
	// 비어 있는가"를 묻고 로컬 refs까지 보는데, 여기서 로컬 브랜치는 전환이 지울
	// 대상이다. 그것을 이유로 막으면 게이트가 자기가 치울 것을 근거로 거부한다.
	// 이쪽 질문은 "정리 후에도 비어 있을 것인가"이고 답은 원격에만 있다.
	//
	// 원격 브랜치는 provider가 이슈에 연결한 것이므로 switch-mode가 지우지
	// 않는다. #163이 정한 순서대로 orca 준비 뒤에 `gh issue develop`을 다시
	// 붙이는 것이 이 상태를 푸는 경로다.
	if requested == string(issueops.ExecutionModeOrca) && inventory.Branch != "" {
		if code, _ := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/remotes/origin/"+inventory.Branch); code == 0 {
			facts.OrcaRemoteBranchExists = true
			result.BranchFreeError = fmt.Sprintf(
				"branch %q still exists on origin, so Orca would take a suffixed name after the switch: "+
					"delete the remote branch if it holds no work, run `git fetch --prune` if it is already gone, "+
					"or prepare Orca first and re-attach the linked branch afterwards",
				inventory.Branch)
		}
	}
	return inventory, modeswitchdomain.MissingGates(facts)
}

// removeSwitchModeWorkspace는 워크트리와 로컬 브랜치를 지운다. 원격은 건드리지
// 않는다 — provider-linked 브랜치는 이슈 연결을 담고 있고, 새 모드의 준비가 그
// 이름을 다시 쓴다.
type switchModeEffects struct {
	stateRoot string
	deps      ExecutionSwitchModeDependencies
}

func (e *switchModeEffects) RemoveWorktree(_ context.Context, repo, root string) error {
	if code, out := e.deps.Git(repo, "worktree", "remove", "--force", root); code != 0 {
		return fmt.Errorf("switch-mode could not remove the canonical worktree (record preserved): %s", strings.TrimSpace(out))
	}
	return nil
}

func (e *switchModeEffects) RemoveBranch(_ context.Context, repo, branch string) error {
	if code, out := e.deps.Git(repo, "branch", "-D", branch); code != 0 {
		return fmt.Errorf("switch-mode could not remove the local branch (record preserved): %s", strings.TrimSpace(out))
	}
	return nil
}

func (e *switchModeEffects) ResetExecution(ctx context.Context, id, expectedSHA string) error {
	// 외부 Git 조작 뒤에는 다시 공통 span lock에서 fence와 record CAS를 확인한다.
	// cleanup abandon이 먼저 arm됐다면 그 receipt를 지우지 않고 record를 보존한다.
	return withIssueOpsLock(ctx, e.stateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(e.stateRoot, id)
		if err != nil {
			return err
		}
		currentSHA, err := hashJSON(current)
		if err != nil {
			return err
		}
		if currentSHA != expectedSHA {
			return fmt.Errorf("execution switch-mode authority changed before record mutation")
		}
		current.Execution = nil
		current.WorktreePath = ""
		current.PlanPath = ""
		_, err = writeIssueOps(e.stateRoot, current)
		return err
	})
}
