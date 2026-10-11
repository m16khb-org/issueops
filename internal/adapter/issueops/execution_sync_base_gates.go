package issueops

import (
	"context"
	"strings"

	"issueops/internal/contract/issueops"
	basesyncdomain "issueops/internal/domain/issueopsbasesync"
)

// executionSyncBaseGateRun은 게이트 평가 중 inventory·result·missing을 함께
// 누적하는 작업 상태다. 관측 메서드는 executionSyncBaseGates가 정한 순서(⑥ 헤드 확인이 ③보다 먼저)로 호출된다.
type executionSyncBaseGateRun struct {
	ctx       context.Context
	deps      issueops.ExecutionSyncBaseDeps
	mode      string
	inventory executionSyncBaseInventory
	result    *issueops.ExecutionSyncBaseResult
	missing   []string
}

func fillExecutionSyncBaseInventory(inventory *executionSyncBaseInventory, record issueops.IssueOpsRecord, execution *issueops.Execution) {
	inventory.PendingIntentAbsent = execution.Pending == nil
	inventory.LeaseGeneration = execution.Lease.Generation
	inventory.LeaseStatus = string(execution.Lease.Status)
	if execution.Completion != nil {
		inventory.CompletionGeneration = execution.Completion.Generation
		inventory.CompletionFinalHead = execution.Completion.FinalHead
	}
	inventory.Branch = strings.TrimSpace(execution.Workspace.Branch)
	inventory.Root = strings.TrimSpace(execution.Workspace.Root)
	if record.BranchPrepare != nil {
		inventory.BaseBranch = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
}

func executionSyncBaseRecordFacts(record issueops.IssueOpsRecord, execution *issueops.Execution, inventory executionSyncBaseInventory) basesyncdomain.RecordGateFacts {
	facts := basesyncdomain.RecordGateFacts{
		ExecutionPresent: true, LeaseActive: execution.Lease.Status == issueops.LeaseStatusActive,
		CompletionPresent: execution.Completion != nil, RemoteArtifactPresent: record.RemoteArtifact != nil,
		PendingIntentAbsent: inventory.PendingIntentAbsent, BaseBranchPresent: inventory.BaseBranch != "",
	}
	if execution.Completion != nil {
		facts.CompletionGeneration = execution.Completion.Generation
	}
	return facts
}

// resolveOID는 git 출력이 비어 있지 않은 OID일 때만 성공으로 돌려준다.
func (g *executionSyncBaseGateRun) resolveOID(args ...string) (string, bool) {
	code, oid := g.deps.Git(g.ctx, g.inventory.Root, args...)
	oid = strings.TrimSpace(oid)
	return oid, code == 0 && oid != ""
}

func (g *executionSyncBaseGateRun) checkHeadOnRecordedBranch() {
	if code, head := g.deps.Git(g.ctx, g.inventory.Root, "branch", "--show-current"); code != 0 ||
		g.inventory.Branch == "" || strings.TrimSpace(head) != g.inventory.Branch {
		g.missing = append(g.missing, "head_on_recorded_branch")
	}
}

// observeMergeState는 ⑦ merge_state_clean / merge_in_progress: 중간 상태를
// 모드별로 갈라 본다(design-review F11 — apply는 거부, finalize/abort는 필수 전제).
func (g *executionSyncBaseGateRun) observeMergeState() {
	g.inventory.MergeInProgress = executionSyncBaseMergeInProgress(g.ctx, g.inventory.Root, g.deps)
	g.result.MergeInProgress = g.inventory.MergeInProgress
	mergeFacts := basesyncdomain.MergeStateFacts{Mode: g.mode, MergeInProgress: g.inventory.MergeInProgress}
	if g.mode == issueops.ExecutionSyncBaseApply {
		// ⑧ worktree_clean: tracked 변경만 차단하고 untracked는 경고로
		//    나열한다(design-review F10 — 상시 거부 방지).
		trackedDirty, untracked := executionSyncBaseWorktreeStatus(g.ctx, g.inventory.Root, g.deps)
		g.result.UntrackedWarnings = untracked
		mergeFacts.TrackedDirty = trackedDirty
	}
	g.missing = append(g.missing, basesyncdomain.MissingMergeStateGates(mergeFacts)...)
}

func (g *executionSyncBaseGateRun) observeWorkTip() {
	if oid, ok := g.resolveOID("rev-parse", "HEAD"); ok {
		g.inventory.WorkOID = oid
	} else {
		g.missing = append(g.missing, "work_tip_resolved")
	}
	g.result.WorkOID = g.inventory.WorkOID
}

// observeRemoteBranch는 ③ remote_branch_present: 머지·삭제된 원격 브랜치
// 부활을 방지한다(design-review F7).
func (g *executionSyncBaseGateRun) observeRemoteBranch() {
	if code, out := g.deps.Git(g.ctx, g.inventory.Root, "ls-remote", "--heads", "origin", "refs/heads/"+g.inventory.Branch); code != 0 {
		g.missing = append(g.missing, "remote_branch_readable")
	} else if fields := strings.Fields(strings.TrimSpace(out)); len(fields) > 0 {
		g.inventory.RemoteBranchPresent = true
		g.inventory.RemoteWorkOID = fields[0]
	}
	if !g.inventory.RemoteBranchPresent {
		g.missing = append(g.missing, "remote_branch_present")
	}
	g.result.RemoteBranchPresent = g.inventory.RemoteBranchPresent
}

// resolveBaseTip은 모드별로 base tip을 확정한다.
func (g *executionSyncBaseGateRun) resolveBaseTip() {
	switch g.mode {
	case issueops.ExecutionSyncBasePreview, issueops.ExecutionSyncBaseApply:
		g.fetchBaseTip()
	case issueops.ExecutionSyncBaseFinalize:
		// finalize는 진행 중인 머지를 마무리한다 — 대상 base tip은 MERGE_HEAD가
		// 그대로 들고 있다. 재fetch하면 진행 중 머지의 base가 바뀐 값으로 기록될
		// 수 있으므로 네트워크를 건드리지 않고 MERGE_HEAD로 확정한다.
		if oid, ok := g.resolveOID("rev-parse", "MERGE_HEAD"); ok {
			g.inventory.BaseOID = oid
		} else {
			g.missing = append(g.missing, "merge_head_resolved")
		}
	}
}

// fetchBaseTip은 fetch 선행(preview·apply)으로 stale base 머지를 방지한다
// (design-review F6 — pr-readiness strict 선례). base tip은 반드시 fetch 이후 값이어야 한다.
func (g *executionSyncBaseGateRun) fetchBaseTip() {
	if g.inventory.BaseBranch == "" {
		return
	}
	if code, _ := g.deps.Git(g.ctx, g.inventory.Root, "fetch", "--quiet", "origin", g.inventory.BaseBranch); code != 0 {
		g.missing = append(g.missing, "base_fetch")
	} else if oid, ok := g.resolveOID("rev-parse", "FETCH_HEAD"); ok {
		g.inventory.BaseOID = oid
	} else {
		g.missing = append(g.missing, "base_tip_resolved")
	}
}

// observeAncestry는 병합 필요성과 push 재시도 필요성(ahead)을 관측해 preview가 보고하게 한다.
func (g *executionSyncBaseGateRun) observeAncestry() {
	inv := g.inventory
	if inv.BaseOID != "" && inv.WorkOID != "" {
		if code, _ := g.deps.Git(g.ctx, inv.Root, "merge-base", "--is-ancestor", inv.BaseOID, inv.WorkOID); code != 0 {
			g.result.MergeNeeded = true
		}
	}
	if inv.RemoteWorkOID != "" && inv.WorkOID != "" && !strings.EqualFold(inv.RemoteWorkOID, inv.WorkOID) {
		if code, _ := g.deps.Git(g.ctx, inv.Root, "merge-base", "--is-ancestor", inv.RemoteWorkOID, inv.WorkOID); code == 0 {
			g.result.PushRetryRequired = true
		}
	}
}

// predictConflicts는 preview에서 예상 충돌 파일을 노출한다. merge-tree 미지원
// (git 2.38 미만)은 fail-closed로 거부한다 — 예측 없는 apply 안내는 만들지 않는다.
func (g *executionSyncBaseGateRun) predictConflicts() {
	inv := g.inventory
	if g.mode != issueops.ExecutionSyncBasePreview || !g.result.MergeNeeded || inv.BaseOID == "" || inv.WorkOID == "" {
		return
	}
	conflicts, err := executionSyncBasePredictConflicts(g.ctx, inv.Root, inv.WorkOID, inv.BaseOID, g.deps)
	if err != nil {
		g.missing = append(g.missing, "merge_tree_supported")
	} else {
		g.result.ConflictFiles = conflicts
	}
}
