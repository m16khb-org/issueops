package issueops

import (
	"context"
	"fmt"
	"os"
	"strings"

	"issueops/internal/adapter/issueops/pathutil"
	abandonapp "issueops/internal/application/issueopscleanup"
	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	abandondomain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	reconciledomain "issueops/internal/domain/issueopsreconcile"
	"issueops/internal/port"
)

// CleanupAbandonRuntime는 게이트 평가의 외부 표면이다.
//
// Orca는 pending_intent_safe 게이트가 sealed marker로 orca 인벤토리를 실조회할
// 때 쓴다. 레코드의 Failure.Code("external_operation_ambiguous")는 5가지 상이한
// 애매성 경로에서 동일하게 기록되므로(execution_orca_intent.go:113-157) 레코드
// 만으로는 "orca에 아무것도 없음"을 증명할 수 없다. 어댑터 부재는 통과가
// 아니라 거부다(design-review 라운드 2 차단 1).
type CleanupAbandonRuntime struct {
	StateRoot string
	Git       func(dir string, args ...string) (int, string)
	Orca      port.ExecutionOrcaProvisioner
	// OrcaOwner는 게이트 ⑨가 orca 자원 잔여를 실조회할 때 쓴다. 게이트 ⑥은
	// 로컬 디렉터리만 보므로 orca 레지스트리에 남은 task를 놓친다(#136).
	//
	// Orca와 같은 이유로 부재는 통과가 아니라 거부다. 다만 조회 자체는 orca
	// 바인딩이 있는 레코드에서만 일어난다 — direct 사이클까지 어댑터를
	// 요구하면 아무 관련 없는 정리가 막힌다.
	OrcaOwner port.ExecutionOrcaOwnerInspector
	// Processes와 OrcaTerminals는 finish와 같은 apply ①′(점유 프로세스·Orca
	// 터미널 종료) 표면이다(#477).
	Processes     CleanupProcessDeps
	OrcaTerminals port.CleanupOrcaTerminals
	// Remote는 원격 효과 플래그(--close-pr, --close-issue,
	// --delete-remote-branch)가 있을 때만 쓰인다. 플래그 없이 폐기하면 이
	// 필드는 nil이어도 되고 원격은 전혀 조회되지 않는다. 플래그가 있는데
	// 없으면 게이트가 막는다 — 어댑터 부재는 통과가 아니라 거부다.
	Remote port.IssueProvider
}

// cleanupAbandonGates observes resources; the domain decides whether those facts
// permit abandonment and preserves the published diagnostic order.
func cleanupAbandonGates(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, req CleanupAbandonRequest, deps CleanupAbandonRuntime, result *CleanupAbandonResult) (issueops.CleanupAbandonInventory, []string) {
	inventory := abandondomain.CleanupAbandonTargets(record)
	inventory.RecordSHA = abandonapp.CleanupAbandonRecordSHA(record)
	observed := abandondomain.CleanupAbandonObservation{ResolvedChildren: cleanupAbandonResolvedChildren(stateRoot, record)}
	if record.Execution != nil {
		observed.LeaseHolderless = !leasedomain.LeaseHoldsWriter(string(record.Execution.Lease.Status))
	}
	if linked := strings.TrimSpace(record.WorktreePath); linked != "" {
		observed.WorktreeIdentityConflict = pathutil.CleanAbsPath(inventory.WorktreeRoot) != pathutil.CleanAbsPath(linked)
	}
	if inventory.WorktreeRoot != "" {
		info, err := os.Lstat(inventory.WorktreeRoot)
		switch {
		case os.IsNotExist(err):
		case err != nil:
			observed.WorktreeUnobservable = true
		case !info.IsDir():
			observed.WorktreeUnobservable = true
		default:
			inventory.WorktreePresent = true
		}
	}
	if inventory.WorktreePresent {
		inventory, observed.WorktreeHeadObservable = cleanupAbandonInspectWorktree(inventory, deps)
		workspace, missing := NewCleanupWorkspaceCleaner(deps.Processes, deps.OrcaTerminals).Observe(ctx, record, inventory.WorktreeRoot)
		observed.WorkspaceMissing = missing
		observed.Occupants = workspace.Occupants
		inventory.WorkspaceProcesses = workspace.Receipts
		inventory.OrcaTerminals = workspace.Terminals
		inventory.OrcaAppPID = workspace.AppPID
		inventory.OrcaRuntimeReady = workspace.RuntimeReady
	}
	if inventory.Branch != "" {
		code, out := deps.Git(record.Repo, "rev-parse", "--verify", "--quiet", "refs/heads/"+inventory.Branch)
		observed.BranchObservable = code == 1 || (code == 0 && strings.TrimSpace(out) != "")
		if code == 0 {
			inventory.BranchOID = strings.TrimSpace(out)
		}
	}
	if inventory.BranchOID != "" {
		code, out := deps.Git(record.Repo, "worktree", "list", "--porcelain")
		observed.RegistryObservable = code == 0
		if code == 0 {
			inventory.BranchCheckoutPath = cleanupAbandonBranchCheckoutPath(out, inventory.Branch)
			observed.BranchCheckedOutElsewhere = inventory.BranchCheckoutPath != "" && !samePath(inventory.BranchCheckoutPath, inventory.WorktreeRoot)
		}
	}
	if failure := record.CleanupAbandonFailure; failure != nil {
		observed.FailureEvidence.SameWorktree = samePath(failure.WorktreePath, inventory.WorktreeRoot)
		observed.FailureEvidence.SealMatches = failure.InventorySHA256 == abandonapp.CleanupAbandonFailureSeal(record, failure)
	}
	if record.Execution != nil && record.Execution.Pending != nil {
		if err := cleanupAbandonPendingSafe(ctx, stateRoot, record, inventory, deps); err != nil {
			observed.PendingIntentError = cleanupAbandonPendingRecovery(record.ID, err)
		}
	}
	if err := cleanupAbandonOrcaResourcesAbsent(ctx, record, deps, inventory.WorktreePresent && len(inventory.OrcaTerminals) > 0); err != nil {
		observed.OrcaResidueError = err.Error()
	}
	remoteObserver := abandonapp.AbandonRemoteObserver{RemoteRef: (LinkedBranchRemoteRef{RunGit: func(_ context.Context, dir string, args ...string) (int, string) {
		return deps.Git(dir, args...)
	}}).Observe}
	inventory, remote := remoteObserver.Observe(ctx, record, req, deps.Remote, inventory)
	observed.RemoteMissing = remote.RemoteMissing
	observed.RemoteEffects = remote.RemoteEffects
	observed.RemoteArtifactState = remote.RemoteArtifactState
	observed.IssueState = remote.IssueState
	inventory, *result = abandondomain.BuildCleanupAbandonPreview(record, req, inventory, observed)
	return inventory, result.Missing
}

func cleanupAbandonBranchCheckoutPath(output, branch string) string {
	currentPath := ""
	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		case strings.TrimSpace(line) == "branch refs/heads/"+branch:
			return currentPath
		}
	}
	return ""
}

func cleanupAbandonInspectWorktree(inventory issueops.CleanupAbandonInventory, deps CleanupAbandonRuntime) (issueops.CleanupAbandonInventory, bool) {
	if code, out := deps.Git(inventory.WorktreeRoot, "rev-parse", "--show-toplevel"); code == 0 {
		inventory.WorktreeCanonical = samePath(out, inventory.WorktreeRoot)
	}
	if code, out := deps.Git(inventory.WorktreeRoot, "symbolic-ref", "--quiet", "--short", "HEAD"); code == 0 {
		inventory.WorktreeBranch = strings.TrimSpace(out)
	}
	code, out := deps.Git(inventory.WorktreeRoot, "rev-parse", "HEAD")
	headObservable := code == 0
	if headObservable {
		inventory.WorktreeHead = strings.TrimSpace(out)
	}
	if code, out := deps.Git(inventory.WorktreeRoot, "status", "--porcelain=v1"); code == 0 {
		inventory.WorktreeClean = strings.TrimSpace(out) == ""
	}
	return inventory, headObservable
}

// cleanupAbandonPendingRecovery는 pending intent가 안전하다고 증명되지 않았을 때
// reconcile부터 Orca worktree 회수까지 남은 운영 경로를 항상 함께 제시한다.
func cleanupAbandonPendingRecovery(id string, cause error) string {
	detail := cause.Error()
	if strings.Contains(detail, "execution reconcile") && strings.Contains(detail, "worktree") {
		return detail
	}
	return fmt.Sprintf("%s; run `issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon",
		detail, id)
}

// cleanupAbandonOrcaResourcesAbsent는 레코드를 지워도 orca 자원이 소유자를 잃지
// 않는지 판정한다.
//
// abandon은 Orca 자원을 지우지 않는다. orca task가 살아 있는데 레코드를 지우면
// 그 task는 소유자 조회가 영구히 0건이 되어 operational_task_residue로 계속
// 보고된다. 그래서 Orca 잔여물을 지울 수 있는 경로로 보낸다.
//
// 레코드에 바인딩이 있다는 것만으로 막지 않고 실조회한다. 그렇게 하면 orca에서
// 이미 정리된 사이클까지 차단해 중도 포기 경로가 사라진다.
//
// 조회할 수 없으면 통과가 아니라 거부다(#106 pending_intent_safe와 같은 계약).
func cleanupAbandonOrcaResourcesAbsent(ctx context.Context, record issueops.IssueOpsRecord, deps CleanupAbandonRuntime, terminalsReachable bool) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return nil
	}
	binding := *record.Execution.Orca
	if strings.TrimSpace(binding.TaskID) == "" {
		return nil
	}
	if deps.OrcaOwner == nil {
		return fmt.Errorf("Orca owner inspector is not configured; resolve this cycle with `issueops cleanup finish` or `issueops cleanup orphan`")
	}
	// Orca 런타임이 롤오버되면 봉인된 runtime ID로는 아무것도 조회할 수 없고,
	// 어댑터는 bounded 권한 없이 바뀐 runtime의 인벤토리를 돌려주지 않는다. 그
	// 조회 실패를 ambiguous로 취급하면 롤오버를 겪은 레코드는 finish(머지 증적
	// 필요)로도 abandon으로도 은퇴하지 못한다(#342에서 실측: 4건이 이 사유로 차단).
	//
	// 권한은 holderless에서만 연다. 살아 있는 writer가 있으면 이전 런타임의 자원
	// 부재를 증명할 수 없기 때문이다. 게이트 ③이 같은 판정으로 active/revoking을
	// 이미 거부하므로 두 게이트는 같은 방향으로 닫힌다.
	allowRuntimeRollover := !leasedomain.LeaseHoldsWriter(string(record.Execution.Lease.Status)) &&
		record.Execution.Lease.Holder == nil
	inventory, err := deps.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		AllowRuntimeRollover: allowRuntimeRollover,
	})
	if err != nil {
		return fmt.Errorf("Orca owner inventory is ambiguous; resolve this cycle with `issueops cleanup finish` or `issueops cleanup orphan`: %w", err)
	}
	// 터미널 잔여는 apply ①′가 닫을 수 있을 때(워크트리 존재·런타임이 그 워크트리의
	// 터미널을 나열)만 거부 사유가 아니다. 워크트리가 없거나 터미널이 나열되지 않으면
	// ①′는 아무것도 닫지 못하므로 살아 있는 터미널은 소유자 없는 자원이 된다(pr-review
	// #478 F4). task/dispatch 잔여는 항상 거부한다(#477).
	if inventory.TaskLive || (inventory.TerminalLive && !terminalsReachable) {
		return fmt.Errorf("Orca resources are still live (task_status=%q dispatch_status=%q terminal_live=%t terminal_reachable_by_stop=%t); abandon leaves them without an owner, so use `issueops cleanup finish` or `issueops cleanup orphan`",
			inventory.TaskStatus, inventory.DispatchStatus, inventory.TerminalLive, terminalsReachable)
	}
	return nil
}

// cleanupAbandonPendingSafe는 pending external intent를 가진 레코드를 삭제해도
// 안전한지 판정한다. 기본값은 거부이고, 아래 조건 전부를 통과할 때만 허용한다.
//
// owner 단계는 현재 단계만 비어 있다고 충분하지 않다. 예를 들어 dispatch가
// 없더라도 terminal/task가 남을 수 있다. 따라서 worktree부터 현재 단계까지
// 봉인된 인벤토리를 모두 authoritative zero로 확인하고, 별도 게이트 ⑨에서
// 이전 generation의 owner binding도 확인한다.
func cleanupAbandonPendingSafe(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, inventory issueops.CleanupAbandonInventory, deps CleanupAbandonRuntime) error {
	pending := record.Execution.Pending
	// (a) kind allowlist — 로컬 orca mutation 한정. remote PR/MR 계열 kind는
	// 원격 고아 PR을 남길 수 있으므로 무조건 거부하고 reconcile로 보낸다.
	if !reconciledomain.IsOrcaIntentKind(pending.Kind) {
		// reconcile을 지시하는 것만으로는 부족했다. 실측에서 운영자는 reconcile을
		// 완주한 뒤 무엇을 해야 하는지 알 수 없었다(#139) — 남은 절차를 함께
		// 알려야 게이트 응답이 탈출 경로가 된다(#140).
		return fmt.Errorf("pending external intent kind %q is not local-only; run `issueops execution reconcile --id %s --preview --json` until it settles, then reclaim the Orca worktree with `orca worktree remove` before retrying abandon",
			pending.Kind, record.ID)
	}
	if record.Execution.Mode != issueops.ExecutionModeOrca {
		return fmt.Errorf("Orca intent requires an Orca execution record")
	}
	// (b) 기록된 canonical workspace가 디스크에 없어야 한다.
	if inventory.WorktreePresent {
		return fmt.Errorf("recorded workspace root still exists on disk")
	}
	// (c) sealed marker 실조회. 레코드는 "없음"을 증명하지 못한다.
	payload, err := readExternalOrcaIntentPayload(stateRoot, pending.OperationID)
	if err != nil {
		return err
	}
	if payload.LifecycleID != record.ID || payload.Marker != pending.Marker ||
		payload.Generation != record.Execution.Lease.Generation ||
		pending.Kind != preparationdomain.PendingKind(payload.Stage) {
		return fmt.Errorf("Orca external intent row does not belong to this lifecycle")
	}
	if deps.Orca == nil {
		return fmt.Errorf("Orca intent inspector is not configured")
	}
	requests, err := cleanupAbandonIntentInspectionRequests(record, payload)
	if err != nil {
		return err
	}
	for _, request := range requests {
		inspected, inspectErr := deps.Orca.InspectIntent(ctx, request)
		if inspectErr != nil {
			return fmt.Errorf("Orca %s intent inventory is ambiguous; intent retained: %w", request.Stage, inspectErr)
		}
		if len(inspected.Candidates) != 0 {
			return fmt.Errorf("Orca %s intent inventory found %d candidate(s); the mutation may have landed",
				request.Stage, len(inspected.Candidates))
		}
		if !inspected.AuthoritativeZero {
			return fmt.Errorf("Orca %s intent inventory returned a non-authoritative zero", request.Stage)
		}
	}
	return nil
}

func cleanupAbandonIntentInspectionRequests(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) ([]port.ExecutionOrcaIntentRequest, error) {
	current, err := executionOrcaIntentInspectionRequest(record, payload)
	if err != nil {
		return nil, err
	}
	requests := make([]port.ExecutionOrcaIntentRequest, 0, 4)
	worktree := current
	worktree.Stage = port.ExecutionOrcaIntentWorktree
	worktree.Prepared = nil
	worktree.Launch = nil
	worktree.TerminalPTYID = ""
	worktree.RunID = ""
	worktree.RunBound = false
	worktree.TaskID = ""
	requests = append(requests, worktree)
	if payload.Stage == preparationcontract.IntentStageWorktree {
		return requests, nil
	}

	terminal := current
	terminal.Stage = port.ExecutionOrcaIntentTerminal
	terminal.TerminalPTYID = ""
	terminal.RunID = ""
	terminal.RunBound = false
	terminal.TaskID = ""
	requests = append(requests, terminal)
	if payload.Stage == preparationcontract.IntentStageTerminal ||
		payload.Stage == preparationcontract.IntentStageRun ||
		payload.Stage == preparationcontract.IntentStageRunBind {
		return requests, nil
	}

	// Run은 삭제 수단이 없는 경량 namespace라 cleanup residue가 아니다.
	// 생성·바인딩 후보는 무시하고, 실제 실행 자원인 task부터 다시 검사한다.
	task := current
	task.Stage = port.ExecutionOrcaIntentTask
	task.RunBound = true
	task.TaskID = ""
	requests = append(requests, task)
	if payload.Stage == preparationcontract.IntentStageTask {
		return requests, nil
	}
	if payload.Stage != preparationcontract.IntentStageDispatch {
		return nil, fmt.Errorf("unsupported Orca cleanup intent stage %q", payload.Stage)
	}
	return append(requests, current), nil
}

// cleanupAbandonResolvedChildren는 자식 cycle이 끝났음을 **관측**으로 확인한다.
//
// 해소의 근거는 record가 실재하고 phase가 done인 것 하나다. record 부재는
// 근거가 아니다 — 정리돼서 사라진 것인지 유실된 것인지 구분할 수 없고, 모르는
// 것을 끝난 것으로 넘기면 이 게이트가 막으려던 고아가 그대로 생긴다.
//
// 부재한 자식을 정리하려면 그 사실을 부모 record에 남겨야 한다. IssueLinks의
// CloseVerifiedAt이 그 자리이고, `issueops cleanup close-children`이 원격
// 관측으로 그것을 기록한다. abandon 자신은 원격에 닿지 않는 계약이므로
// 여기서 이슈 종료를 직접 확인하지 않는다.
func cleanupAbandonResolvedChildren(stateRoot string, record issueops.IssueOpsRecord) map[string]bool {
	resolved := map[string]bool{}
	for _, child := range record.ChildCycles {
		id := strings.TrimSpace(child.CycleID)
		if id == "" {
			continue
		}
		childRecord, err := ReadIssueOpsExisting(stateRoot, id)
		if err != nil {
			continue
		}
		if childRecord.Phase == IssueOpsPhaseDone {
			resolved[id] = true
		}
	}
	return resolved
}

// Command uses the inherited execution context for all default Git processes.
func (r CleanupAbandonRuntime) Command(ctx context.Context, dir string, args ...string) (int, string) {
	if r.Git != nil {
		return r.Git(dir, args...)
	}
	return defaultExecutionSyncBaseGit(ctx, dir, args...)
}
func (r CleanupAbandonRuntime) Plan(ctx context.Context, record issueops.IssueOpsRecord, req issueops.CleanupAbandonRequest, provider port.IssueProvider) (issueops.CleanupAbandonInventory, issueops.CleanupAbandonResult) {
	deps := r
	deps.Git = func(dir string, args ...string) (int, string) { return r.Command(ctx, dir, args...) }
	deps.Remote = provider
	result := issueops.CleanupAbandonResult{}
	inventory, _ := cleanupAbandonGates(ctx, r.StateRoot, record, req, deps, &result)
	return inventory, result
}
func (r CleanupAbandonRuntime) Stop(ctx context.Context, inventory issueops.CleanupAbandonInventory, processes []issueops.CleanupWorkspaceProcess) ([]issueops.CleanupWorkspaceProcess, int, error) {
	return NewCleanupWorkspaceCleaner(r.Processes, r.OrcaTerminals).Stop(ctx, inventory.WorktreeRoot, processes, inventory.OrcaTerminals, inventory.OrcaRuntimeReady, inventory.OrcaAppPID)
}
