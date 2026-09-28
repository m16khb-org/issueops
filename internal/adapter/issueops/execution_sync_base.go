package issueops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	basesyncapp "issueops/internal/application/issueopsbasesync"
	"issueops/internal/contract/issueops"
	basesyncdomain "issueops/internal/domain/issueopsbasesync"
)

// execution sync-base는 completion 이후에도 남는 typed 충돌 해소 표면이다
// (이슈 #114). sealed git topology 가드 정책은 그대로 두고, lifecycle guard의
// typed control plane 목록과 commandparse spec에만 이 명령을 가산 등록한다.
// typed 등록은 훅의 mutation 가드 블록 전체를 스킵시키므로(design-review F14) lease와
// 권위 검사는 100% 이 파일의 책임이다 — 훅은 --id만 보고 통과시킨다.

// executionSyncBaseGitTimeout은 fetch/push가 자격증명 프롬프트나 네트워크
// 정지에 걸려 holder 세션을 무한정 붙잡는 것을 막는 상한이다(design-review F5 —
// issueops 프로덕션의 첫 push 표면).
const executionSyncBaseGitTimeout = 120 * time.Second

// executionSyncBaseInventory는 fingerprint 입력이 되는 현재 관측 상태다.
// base tip은 fetch 이후 값이어야 stale base 머지를 fingerprint가 잡는다.
type executionSyncBaseInventory struct {
	ID                   string `json:"id"`
	Repo                 string `json:"repo"`
	Branch               string `json:"branch"`
	BaseBranch           string `json:"base_branch"`
	BaseOID              string `json:"base_oid"`
	WorkOID              string `json:"work_oid"`
	RemoteWorkOID        string `json:"remote_work_oid"`
	LeaseGeneration      uint64 `json:"lease_generation"`
	LeaseStatus          string `json:"lease_status"`
	CompletionGeneration uint64 `json:"completion_generation"`
	CompletionFinalHead  string `json:"completion_final_head"`
	PendingIntentAbsent  bool   `json:"pending_intent_absent"`
	MergeInProgress      bool   `json:"merge_in_progress"`
	RemoteBranchPresent  bool   `json:"remote_branch_present"`

	Root string `json:"-"`
}

// SyncExecutionBase는 게이트 10종을 fail-closed로 평가하고, 모드별 절차를
// 실행한다. preview는 워크트리를 오염시키지 않는 관측 전용이며(merge-tree는
// ODB에만 객체를 쓴다), 변형 3모드는 활성 holder 또는 generation이 일치하는
// released current completion의 권위를 요구한다.
func SyncExecutionBase(ctx context.Context, stateRoot string, req ExecutionSyncBaseRequest, deps ExecutionSyncBaseDeps) (ExecutionSyncBaseResult, error) {
	if deps.Git == nil {
		deps.Git = defaultExecutionSyncBaseGit
	}
	mode := strings.TrimSpace(req.Mode)
	switch mode {
	case ExecutionSyncBasePreview, ExecutionSyncBaseApply, ExecutionSyncBaseFinalize, ExecutionSyncBaseAbort:
	default:
		return ExecutionSyncBaseResult{OK: false, ID: req.ID, Mode: mode},
			fmt.Errorf("execution sync-base requires exactly one mode: --preview, --apply, --finalize, or --abort")
	}
	mutating := mode != ExecutionSyncBasePreview
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return ExecutionSyncBaseResult{OK: false, ID: req.ID, Mode: mode}, err
	}
	result := ExecutionSyncBaseResult{OK: true, ID: record.ID, Mode: mode}
	if record.Execution != nil {
		result.LeaseGeneration = record.Execution.Lease.Generation
	}
	// preview는 released 사이클과 비-holder 세션의 진단 채널이므로 actor를
	// 요구하지 않는다. 변형 3모드만 live process receipt까지 정규화한다.
	var actor issueops.NativeActor
	if mutating {
		actor, err = normalizeNativeActor(req.Actor)
		if err != nil {
			result.OK = false
			return result, err
		}
	}
	inventory, missing := executionSyncBaseGates(ctx, record, req, mode, actor, deps, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		result.NextCommand = executionStatusCommandForSyncBase(record.ID)
		return result, fmt.Errorf("execution sync-base %s is not ready: %s", mode, strings.Join(missing, ", "))
	}
	fingerprint, err := executionSyncBaseFingerprint(inventory)
	if err != nil {
		result.OK = false
		return result, err
	}
	result.Fingerprint = fingerprint
	if mode == ExecutionSyncBasePreview {
		result.NextCommand = fmt.Sprintf(
			"%s --apply --confirm --fingerprint %s ACTOR_FLAGS --json",
			executionSyncBaseCommandPrefix(record), fingerprint)
		return result, nil
	}
	// --confirm은 파괴적 머지 커밋과 push를 만드는 apply에만 요구한다
	// (설계 v2 4모드 표기 그대로 — finalize/abort는 플래그 자체가 명시 의사다).
	if mode == ExecutionSyncBaseApply && !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("execution sync-base --apply requires --confirm")
	}
	switch mode {
	case ExecutionSyncBaseApply:
		return applyExecutionSyncBase(ctx, stateRoot, record, req, actor, inventory, fingerprint, deps, &result)
	case ExecutionSyncBaseFinalize:
		return finalizeExecutionSyncBase(ctx, stateRoot, record, actor, inventory, deps, &result)
	default:
		return abortExecutionSyncBase(ctx, stateRoot, record, inventory, deps, &result)
	}
}

// executionSyncBaseGates는 설계 v2의 게이트 10종을 순서대로 평가하고 missing을
// 나열한다(fail-closed). 워크트리를 관측할 수 없으면 그 지점에서 끊는다 —
// 이후 git 호출은 전부 의미가 없기 때문이다.
func executionSyncBaseGates(ctx context.Context, record issueops.IssueOpsRecord, req ExecutionSyncBaseRequest, mode string,
	actor issueops.NativeActor, deps ExecutionSyncBaseDeps, result *ExecutionSyncBaseResult) (executionSyncBaseInventory, []string) {
	inventory := executionSyncBaseInventory{ID: record.ID, Repo: record.Repo}
	execution := record.Execution
	if execution == nil {
		return inventory, basesyncdomain.MissingRecordGates(basesyncdomain.RecordGateFacts{})
	}
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
	facts := basesyncdomain.RecordGateFacts{
		ExecutionPresent: true, LeaseActive: execution.Lease.Status == issueops.LeaseStatusActive,
		CompletionPresent: execution.Completion != nil, RemoteArtifactPresent: record.RemoteArtifact != nil,
		PendingIntentAbsent: inventory.PendingIntentAbsent, BaseBranchPresent: inventory.BaseBranch != "",
	}
	if execution.Completion != nil {
		facts.CompletionGeneration = execution.Completion.Generation
	}
	missing := basesyncdomain.MissingRecordGates(facts)
	result.Branch, result.BaseBranch = inventory.Branch, inventory.BaseBranch
	// ⑤ worktree_present + cwd_canonical: 소스 루트 호출의 훅 사각지대를
	//    봉쇄한다(design-review F2 — complete/claim 선례 준용).
	info, err := os.Lstat(inventory.Root)
	if inventory.Root == "" || err != nil || !info.IsDir() {
		return inventory, append(missing, "worktree_present")
	}
	if !samePath(req.CWD, inventory.Root) {
		missing = append(missing, "cwd_canonical")
	}
	missing = append(missing, executionSyncBaseAuthorityMissing(execution, req, mode, actor)...)
	// ⑥ head_on_recorded_branch: detached HEAD의 무증상 push 실패를 막는다
	//    (design-review F3).
	if code, head := deps.Git(ctx, inventory.Root, "branch", "--show-current"); code != 0 ||
		inventory.Branch == "" || strings.TrimSpace(head) != inventory.Branch {
		missing = append(missing, "head_on_recorded_branch")
	}
	// ③ remote_branch_present: 머지·삭제된 원격 브랜치 부활 방지(design-review F7).
	if code, out := deps.Git(ctx, inventory.Root, "ls-remote", "--heads", "origin", "refs/heads/"+inventory.Branch); code != 0 {
		missing = append(missing, "remote_branch_readable")
	} else if fields := strings.Fields(strings.TrimSpace(out)); len(fields) > 0 {
		inventory.RemoteBranchPresent = true
		inventory.RemoteWorkOID = fields[0]
	}
	if !inventory.RemoteBranchPresent {
		missing = append(missing, "remote_branch_present")
	}
	result.RemoteBranchPresent = inventory.RemoteBranchPresent
	// ⑦ merge_state_clean / merge_in_progress: 중간 상태를 모드별로 갈라 본다
	//    (design-review F11 — apply는 거부, finalize/abort는 필수 전제).
	inventory.MergeInProgress = executionSyncBaseMergeInProgress(ctx, inventory.Root, deps)
	result.MergeInProgress = inventory.MergeInProgress
	mergeFacts := basesyncdomain.MergeStateFacts{Mode: mode, MergeInProgress: inventory.MergeInProgress}
	if mode == ExecutionSyncBaseApply {
		// ⑧ worktree_clean: tracked 변경만 차단하고 untracked는 경고로
		//    나열한다(design-review F10 — 상시 거부 방지).
		trackedDirty, untracked := executionSyncBaseWorktreeStatus(ctx, inventory.Root, deps)
		result.UntrackedWarnings = untracked
		mergeFacts.TrackedDirty = trackedDirty
	}
	missing = append(missing, basesyncdomain.MissingMergeStateGates(mergeFacts)...)
	// fetch 선행(preview·apply): stale base 머지 방지(design-review F6 —
	// pr-readiness strict 선례). base tip은 반드시 fetch 이후 값이어야 한다.
	switch mode {
	case ExecutionSyncBasePreview, ExecutionSyncBaseApply:
		if inventory.BaseBranch != "" {
			if code, _ := deps.Git(ctx, inventory.Root, "fetch", "--quiet", "origin", inventory.BaseBranch); code != 0 {
				missing = append(missing, "base_fetch")
			} else if code, oid := deps.Git(ctx, inventory.Root, "rev-parse", "FETCH_HEAD"); code == 0 && strings.TrimSpace(oid) != "" {
				inventory.BaseOID = strings.TrimSpace(oid)
			} else {
				missing = append(missing, "base_tip_resolved")
			}
		}
	case ExecutionSyncBaseFinalize:
		// finalize는 진행 중인 머지를 마무리한다 — 대상 base tip은 MERGE_HEAD가
		// 그대로 들고 있다. 재fetch하면 진행 중 머지의 base가 바뀐 값으로 기록될
		// 수 있으므로 네트워크를 건드리지 않고 MERGE_HEAD로 확정한다.
		if code, oid := deps.Git(ctx, inventory.Root, "rev-parse", "MERGE_HEAD"); code == 0 && strings.TrimSpace(oid) != "" {
			inventory.BaseOID = strings.TrimSpace(oid)
		} else {
			missing = append(missing, "merge_head_resolved")
		}
	}
	if code, oid := deps.Git(ctx, inventory.Root, "rev-parse", "HEAD"); code == 0 && strings.TrimSpace(oid) != "" {
		inventory.WorkOID = strings.TrimSpace(oid)
	} else {
		missing = append(missing, "work_tip_resolved")
	}
	result.BaseOID, result.WorkOID = inventory.BaseOID, inventory.WorkOID
	// 병합 필요성과 push 재시도 필요성(ahead)을 관측해 preview가 보고한다.
	if inventory.BaseOID != "" && inventory.WorkOID != "" {
		if code, _ := deps.Git(ctx, inventory.Root, "merge-base", "--is-ancestor", inventory.BaseOID, inventory.WorkOID); code != 0 {
			result.MergeNeeded = true
		}
	}
	if inventory.RemoteWorkOID != "" && inventory.WorkOID != "" && !strings.EqualFold(inventory.RemoteWorkOID, inventory.WorkOID) {
		if code, _ := deps.Git(ctx, inventory.Root, "merge-base", "--is-ancestor", inventory.RemoteWorkOID, inventory.WorkOID); code == 0 {
			result.PushRetryRequired = true
		}
	}
	// preview는 예상 충돌 파일을 노출한다. merge-tree 미지원(git 2.38 미만)은
	// fail-closed로 거부한다 — 예측 없는 apply 안내는 만들지 않는다.
	if mode == ExecutionSyncBasePreview && result.MergeNeeded && inventory.BaseOID != "" && inventory.WorkOID != "" {
		conflicts, err := executionSyncBasePredictConflicts(ctx, inventory.Root, inventory.WorkOID, inventory.BaseOID, deps)
		if err != nil {
			missing = append(missing, "merge_tree_supported")
		} else {
			result.ConflictFiles = conflicts
		}
	}
	return inventory, missing
}

func executionSyncBaseAuthorityMissing(execution *issueops.Execution, req ExecutionSyncBaseRequest, mode string, actor issueops.NativeActor) []string {
	if execution == nil {
		return nil
	}
	facts := basesyncdomain.AuthorityFacts{
		Mode: mode, LeaseStatus: string(execution.Lease.Status), LeaseGeneration: execution.Lease.Generation,
		HolderMatches:     sameNativeActor(execution.Lease.Holder, &actor),
		CompletionPresent: execution.Completion != nil, RequestedGeneration: req.CompletionGeneration,
	}
	if execution.Completion != nil {
		facts.CompletionGeneration = execution.Completion.Generation
	}
	if resolution := execution.SyncBaseResolution; resolution != nil {
		facts.Resolution = basesyncdomain.AuthorityResolution{
			Present: true, LeaseGeneration: resolution.Generation, CompletionGeneration: resolution.CompletionGeneration,
			ActorMatches: sameNativeActor(&resolution.Actor, &actor),
		}
	}
	return basesyncdomain.MissingAuthority(facts)
}

// applyExecutionSyncBase는 무충돌이면 merge commit + push로 완결하고, 충돌이면
// 파일을 나열한 채 merge-in-progress로 정지한다(해소 편집은 같은 holder).
// 병합이 이미 반영된 상태에서의 재실행은 merge를 건너뛰고 push만 수행해
// non-fast-forward 거부 이후로 멱등 수렴한다(설계 v2 push 계약).
func applyExecutionSyncBase(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, req ExecutionSyncBaseRequest,
	actor issueops.NativeActor, inventory executionSyncBaseInventory, fingerprint string,
	deps ExecutionSyncBaseDeps, result *ExecutionSyncBaseResult) (ExecutionSyncBaseResult, error) {
	// ⑩ TOCTOU: apply 직전 재계산 일치. 외부 변경이 있었다면 preview 재발급을
	//    요구하고 멈춘다(cleanup finish 선례).
	if req.Fingerprint != fingerprint {
		result.OK = false
		result.NextCommand = executionSyncBasePreviewCommand(record)
		return *result, fmt.Errorf("stale execution sync-base fingerprint; run --preview again and retry with the new value")
	}
	fail := executionSyncBaseFail(record, result)
	outcome, err := basesyncapp.Apply(ctx, basesyncapp.ApplyRequest{
		Push: basesyncapp.PushRequest{
			ID: record.ID, Root: inventory.Root, Branch: inventory.Branch,
			Mode: issueops.ExecutionSyncBaseEventApply, BaseBranch: inventory.BaseBranch,
			BaseOID: inventory.BaseOID, Actor: executionSyncBaseActorLabel(actor),
		},
		WorkOID: inventory.WorkOID, MergeNeeded: result.MergeNeeded,
		Released: record.Execution != nil && record.Execution.Lease.Status == issueops.LeaseStatusReleased,
	}, &executionSyncBaseApplyEffects{
		executionSyncBasePushEffects: executionSyncBasePushEffects{stateRoot: stateRoot, deps: deps},
		actor:                        actor, inventory: inventory,
	})
	result.ConflictFiles = outcome.ConflictFiles
	result.MergeInProgress, result.Merged = outcome.MergeInProgress, outcome.Merged
	result.MergeCommit = outcome.MergeCommit
	result.Pushed, result.PushRetryRequired = outcome.Pushed, outcome.PushRetryRequired
	if err != nil {
		return fail(outcome.FailedStep, err)
	}
	if outcome.ConflictPaused {
		prefix := executionSyncBaseCommandPrefix(record)
		result.NextCommand = prefix + " --finalize ACTOR_FLAGS --json"
		result.AbortCommand = prefix + " --abort ACTOR_FLAGS --json"
	}
	return *result, nil
}

type executionSyncBaseApplyEffects struct {
	executionSyncBasePushEffects
	actor     issueops.NativeActor
	inventory executionSyncBaseInventory
}

func (e *executionSyncBaseApplyEffects) PredictConflicts(ctx context.Context, root, workOID, baseOID string) ([]string, error) {
	return executionSyncBasePredictConflicts(ctx, root, workOID, baseOID, e.deps)
}

func (e *executionSyncBaseApplyEffects) Merge(ctx context.Context, root, baseOID string) (int, string) {
	return e.deps.Git(ctx, root, "merge", "--no-ff", "--no-edit", baseOID)
}

func (e *executionSyncBaseApplyEffects) UnmergedPaths(ctx context.Context, root string) []string {
	return executionSyncBaseUnmergedPaths(ctx, root, e.deps)
}

func (e *executionSyncBaseApplyEffects) StartResolution(ctx context.Context, id string, conflicts []string) error {
	return startExecutionSyncBaseResolution(ctx, e.stateRoot, id, e.actor, e.inventory, conflicts)
}

func (e *executionSyncBaseApplyEffects) AbortMerge(ctx context.Context, root string) (int, string) {
	return e.deps.Git(ctx, root, "merge", "--abort")
}

func (e *executionSyncBaseApplyEffects) Head(ctx context.Context, root string) (int, string) {
	return e.deps.Git(ctx, root, "rev-parse", "HEAD")
}

func (e *executionSyncBaseApplyEffects) Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// finalizeExecutionSyncBase는 해소가 끝난 merge-in-progress를 커밋하고 push한다.
// unmerged 인덱스 항목이나 충돌 마커가 남아 있으면 커밋하지 않는다.
func finalizeExecutionSyncBase(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, actor issueops.NativeActor,
	inventory executionSyncBaseInventory, deps ExecutionSyncBaseDeps, result *ExecutionSyncBaseResult) (ExecutionSyncBaseResult, error) {
	fail := executionSyncBaseFail(record, result)
	outcome, err := basesyncapp.Finalize(ctx, basesyncapp.FinalizeRequest{Push: basesyncapp.PushRequest{
		ID: record.ID, Root: inventory.Root, Branch: inventory.Branch,
		Mode: issueops.ExecutionSyncBaseEventFinalize, BaseBranch: inventory.BaseBranch, BaseOID: inventory.BaseOID,
		Actor: executionSyncBaseActorLabel(actor),
	}}, &executionSyncBaseFinalizeEffects{executionSyncBasePushEffects{stateRoot: stateRoot, deps: deps}})
	result.ConflictFiles = outcome.ConflictFiles
	result.MergeCommit = outcome.MergeCommit
	result.Merged = outcome.Merged
	if outcome.Merged {
		result.MergeInProgress = false
	}
	result.Pushed, result.PushRetryRequired = outcome.Pushed, outcome.PushRetryRequired
	if err != nil {
		if outcome.DirectError {
			result.OK = false
			result.Missing = append(result.Missing, outcome.Missing)
			return *result, err
		}
		return fail(outcome.FailedStep, err)
	}
	return *result, nil
}

type executionSyncBaseFinalizeEffects struct{ executionSyncBasePushEffects }

func (e *executionSyncBaseFinalizeEffects) ConflictCount(ctx context.Context, root string) int {
	return executionSyncBaseMergeMsgConflictCount(ctx, root, e.deps)
}

func (e *executionSyncBaseFinalizeEffects) UnmergedPaths(ctx context.Context, root string) []string {
	return executionSyncBaseUnmergedPaths(ctx, root, e.deps)
}

func (e *executionSyncBaseFinalizeEffects) CheckStaged(ctx context.Context, root string) (int, string) {
	return e.deps.Git(ctx, root, "diff", "--cached", "--check")
}

func (e *executionSyncBaseFinalizeEffects) Commit(ctx context.Context, root string) (int, string) {
	return e.deps.Git(ctx, root, "commit", "--no-edit")
}

func (e *executionSyncBaseFinalizeEffects) Head(ctx context.Context, root string) (int, string) {
	return e.deps.Git(ctx, root, "rev-parse", "HEAD")
}

func (e *executionSyncBaseFinalizeEffects) Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// abortExecutionSyncBase는 진행 중 머지를 명시적으로 철회한다. 되돌림이므로
// durable 이벤트를 남기지 않는다(이벤트는 apply/finalize 성공 전용).
func abortExecutionSyncBase(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, inventory executionSyncBaseInventory,
	deps ExecutionSyncBaseDeps, result *ExecutionSyncBaseResult) (ExecutionSyncBaseResult, error) {
	fail := executionSyncBaseFail(record, result)
	outcome, err := basesyncapp.Abort(ctx, basesyncapp.AbortRequest{
		ID: record.ID, Root: inventory.Root,
		Released: record.Execution != nil && record.Execution.Lease.Status == issueops.LeaseStatusReleased,
	}, &executionSyncBaseAbortEffects{stateRoot: stateRoot, deps: deps})
	if err != nil {
		return fail(outcome.FailedStep, err)
	}
	result.Aborted, result.MergeInProgress = outcome.Aborted, false
	return *result, nil
}

type executionSyncBaseAbortEffects struct {
	stateRoot string
	deps      ExecutionSyncBaseDeps
}

func (e *executionSyncBaseAbortEffects) AbortMerge(ctx context.Context, root string) (int, string) {
	return e.deps.Git(ctx, root, "merge", "--abort")
}

func (e *executionSyncBaseAbortEffects) ClearResolution(ctx context.Context, id string) error {
	return clearExecutionSyncBaseResolution(ctx, e.stateRoot, id)
}

type executionSyncBasePushEffects struct {
	stateRoot string
	deps      ExecutionSyncBaseDeps
}

func (e *executionSyncBasePushEffects) Push(ctx context.Context, root, refspec string) (int, string) {
	return e.deps.Git(ctx, root, "push", "origin", refspec)
}

func (e *executionSyncBasePushEffects) AppendEvent(ctx context.Context, id string, event issueops.ExecutionSyncBaseEvent) error {
	return appendExecutionSyncBaseEvent(ctx, e.stateRoot, id, event)
}

// appendExecutionSyncBaseEvent는 레코드 쓰기 락 안에서 이벤트만 append한다.
// Completion.FinalHead는 여기서도 다른 어디서도 건드리지 않는다 — 완결 시점
// 증거를 보존하고 merge OID는 이벤트가 담당한다는 정책이다(design-review F9).
func appendExecutionSyncBaseEvent(ctx context.Context, stateRoot, id string, event issueops.ExecutionSyncBaseEvent) error {
	return withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		rec, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if err := basesyncdomain.ValidateEventAppend(rec.Execution != nil); err != nil {
			return err
		}
		rec.Execution.SyncBaseEvents = append(rec.Execution.SyncBaseEvents, event)
		rec.Execution.SyncBaseResolution = nil
		rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_, err = writeIssueOps(stateRoot, rec)
		return err
	})
}

func startExecutionSyncBaseResolution(ctx context.Context, stateRoot, id string, actor issueops.NativeActor,
	inventory executionSyncBaseInventory, conflictFiles []string) error {
	return withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		state := executionSyncBaseResolutionState(record.Execution)
		generations, err := basesyncdomain.BeginResolution(state)
		if err != nil {
			return err
		}
		record.Execution.SyncBaseResolution = &issueops.ExecutionSyncBaseResolution{
			Generation: generations.Lease, CompletionGeneration: generations.Completion,
			BaseOID: inventory.BaseOID, Actor: actor,
			ConflictFiles: append([]string(nil), conflictFiles...), StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}
		record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func clearExecutionSyncBaseResolution(ctx context.Context, stateRoot, id string) error {
	return withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if err := basesyncdomain.ValidateResolutionClear(executionSyncBaseResolutionState(record.Execution)); err != nil {
			return err
		}
		record.Execution.SyncBaseResolution = nil
		record.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_, err = writeIssueOps(stateRoot, record)
		return err
	})
}

func executionSyncBaseResolutionState(execution *issueops.Execution) basesyncdomain.ResolutionState {
	if execution == nil {
		return basesyncdomain.ResolutionState{}
	}
	state := basesyncdomain.ResolutionState{
		ExecutionPresent: true, ResolutionPresent: execution.SyncBaseResolution != nil,
		LeaseGeneration: execution.Lease.Generation,
	}
	if execution.Completion != nil {
		state.CompletionPresent = true
		state.CompletionGeneration = execution.Completion.Generation
	}
	return state
}

func executionSyncBaseFail(record issueops.IssueOpsRecord, result *ExecutionSyncBaseResult) func(string, error) (ExecutionSyncBaseResult, error) {
	return func(step string, stepErr error) (ExecutionSyncBaseResult, error) {
		result.OK = false
		result.FailedStep = step
		result.NextCommand = executionSyncBasePreviewCommand(record)
		return *result, fmt.Errorf("execution sync-base step %s failed (re-run --preview then retry): %w", step, stepErr)
	}
}

// executionSyncBasePredictConflicts는 워크트리를 오염시키지 않고 병합 결과를
// 시험한다(ODB에만 객체를 쓴다 — design-review F12). exit 0=무충돌, 1=충돌,
// 그 외=미지원/오류로 갈라 fail-closed 처리한다(git 2.38 미만 포함).
func executionSyncBasePredictConflicts(ctx context.Context, root, workOID, baseOID string, deps ExecutionSyncBaseDeps) ([]string, error) {
	code, out := deps.Git(ctx, root, "merge-tree", "--write-tree", "--name-only", "-z", workOID, baseOID)
	switch code {
	case 0:
		return nil, nil
	case 1:
		return parseExecutionSyncBaseMergeTreeNames(out), nil
	default:
		return nil, fmt.Errorf("git merge-tree --write-tree is unavailable or failed (git 2.38+ required): %s", strings.TrimSpace(out))
	}
}

// parseExecutionSyncBaseMergeTreeNames는 -z 출력을 파싱한다: 첫 항목은 tree
// OID이고, 그 뒤 빈 항목(섹션 경계)까지가 충돌 파일 목록이며 이후는 사람용
// 메시지다.
func parseExecutionSyncBaseMergeTreeNames(out string) []string {
	names := []string{}
	for i, part := range strings.Split(out, "\x00") {
		if i == 0 {
			continue
		}
		if part == "" {
			break
		}
		names = append(names, part)
	}
	return names
}

func executionSyncBaseUnmergedPaths(ctx context.Context, root string, deps ExecutionSyncBaseDeps) []string {
	code, out := deps.Git(ctx, root, "ls-files", "--unmerged", "-z")
	if code != 0 {
		return nil
	}
	seen := map[string]bool{}
	paths := []string{}
	for _, entry := range strings.Split(out, "\x00") {
		if strings.TrimSpace(entry) == "" {
			continue
		}
		// "<mode> <oid> <stage>\t<path>"
		tab := strings.Index(entry, "\t")
		if tab < 0 {
			continue
		}
		path := entry[tab+1:]
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths
}

// executionSyncBaseMergeInProgress는 MERGE_HEAD/CHERRY_PICK_HEAD/REBASE_HEAD와
// rebase 디렉토리를 모두 본다(design-review F11). 경로 해석 실패는 진행 중으로 본다.
func executionSyncBaseMergeInProgress(ctx context.Context, root string, deps ExecutionSyncBaseDeps) bool {
	for _, ref := range []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REBASE_HEAD"} {
		if code, _ := deps.Git(ctx, root, "rev-parse", "--verify", "--quiet", ref); code == 0 {
			return true
		}
	}
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		code, out := deps.Git(ctx, root, "rev-parse", "--git-path", name)
		if code != 0 {
			return true
		}
		path := executionSyncBaseResolveGitPath(root, out)
		if path == "" {
			continue
		}
		if _, err := os.Lstat(path); err == nil {
			return true
		}
	}
	return false
}

// executionSyncBaseWorktreeStatus는 tracked 오염 여부와 untracked 목록을
// 나눠 돌려준다. status를 읽지 못하면 오염으로 간주한다(fail-closed).
func executionSyncBaseWorktreeStatus(ctx context.Context, root string, deps ExecutionSyncBaseDeps) (bool, []string) {
	code, out := deps.Git(ctx, root, "status", "--porcelain=v1", "--untracked-files=all")
	if code != 0 {
		return true, nil
	}
	trackedDirty := false
	untracked := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		if strings.HasPrefix(line, "?? ") {
			untracked = append(untracked, strings.TrimSpace(line[3:]))
			continue
		}
		trackedDirty = true
	}
	return trackedDirty, untracked
}

// executionSyncBaseMergeMsgConflictCount는 finalize 시점에 남은 유일한 충돌
// 흔적인 MERGE_MSG의 "Conflicts:" 블록을 센다. 해소 후 인덱스에는 흔적이
// 없으므로 다른 관측 경로가 없다 — 실패는 0으로 강등하고 게이트하지 않는다.
func executionSyncBaseMergeMsgConflictCount(ctx context.Context, root string, deps ExecutionSyncBaseDeps) int {
	code, out := deps.Git(ctx, root, "rev-parse", "--git-path", "MERGE_MSG")
	if code != 0 {
		return 0
	}
	path := executionSyncBaseResolveGitPath(root, out)
	if path == "" {
		return 0
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count, inBlock := 0, false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if trimmed == "Conflicts:" {
			inBlock = true
			continue
		}
		if !inBlock {
			continue
		}
		if !strings.HasPrefix(line, "\t") && !strings.HasPrefix(line, "#\t") {
			break
		}
		count++
	}
	return count
}

func executionSyncBaseResolveGitPath(root, raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	return path
}

func executionSyncBaseFingerprint(inventory executionSyncBaseInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func executionSyncBaseActorLabel(actor issueops.NativeActor) string {
	label := strings.TrimSpace(actor.Host) + "/" + strings.TrimSpace(actor.SessionID)
	if agent := strings.TrimSpace(actor.AgentID); agent != "" {
		label += "/" + agent
	}
	return label
}

func executionSyncBaseCommandPrefix(record issueops.IssueOpsRecord) string {
	command := "issueops execution sync-base --id " + quoteExecutionOwnerArg(record.ID)
	if record.Execution != nil && record.Execution.Lease.Status == issueops.LeaseStatusReleased &&
		record.Execution.Completion != nil && record.Execution.Completion.Generation != 0 {
		command += " --completion-generation " + strconv.FormatUint(record.Execution.Completion.Generation, 10)
	}
	return command
}

func executionSyncBasePreviewCommand(record issueops.IssueOpsRecord) string {
	return executionSyncBaseCommandPrefix(record) + " --preview --json"
}

func executionStatusCommandForSyncBase(id string) string {
	return fmt.Sprintf("issueops execution status --id %s --json", id)
}

// defaultExecutionSyncBaseGit은 push/fetch 계약을 강제한다: 자격증명 프롬프트
// 금지(GIT_TERMINAL_PROMPT=0, GIT_ASKPASS 비움), ssh 비대화(BatchMode), 그리고
// context timeout. 이 계약이 없으면 첫 프로덕션 push가 holder 세션을 붙잡는다
// (design-review F5). preflight.GitCmd는 env/ctx를 받지 않아 여기서 직접 실행한다.
func defaultExecutionSyncBaseGit(ctx context.Context, dir string, args ...string) (int, string) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, executionSyncBaseGitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=",
		"SSH_ASKPASS=",
		"GCM_INTERACTIVE=never",
		"GIT_SSH_COMMAND=ssh -oBatchMode=yes",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
			stderr.WriteString(err.Error())
		}
	}
	// merge-tree는 exit 1에서도 stdout에 결과를 담으므로 stdout을 우선한다.
	if code != 0 && strings.TrimSpace(stdout.String()) == "" {
		return code, stderr.String()
	}
	return code, stdout.String()
}
