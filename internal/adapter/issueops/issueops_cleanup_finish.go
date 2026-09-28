package issueops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	completionapp "issueops/internal/application/issueopsremote"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// CleanupFinishDeps는 파괴 단계의 외부 표면 주입점이다. Git은 (dir, args...)를
// 실행해 (exitCode, stdout)을 돌려준다. RemoveOrcaWorktree는 orca 관리
// 워크스페이스 회수이며 "이미 없음"을 성공으로 정규화해야 한다(멱등 계약).
// ReflectAudit는 ④' 감사 라인의 멱등 병합(UpdateIssueBodySection 재사용)이다.
type CleanupFinishDeps struct {
	Git                func(dir string, args ...string) (int, string)
	RemoveOrcaWorktree func(ctx context.Context, worktreeID string) error
	// ReflectAudit는 ②(파괴 시작) 이전에 스냅샷한 completion payload에 감사
	// 라인을 더해 멱등 병합한다 — 삭제된 워크트리를 다시 읽어 보존 본문을
	// 빈 값으로 덮어쓰는 사고를 구조적으로 차단한다(C2-F1 (c)).
	ReflectAudit func(record issueops.IssueOpsRecord, completion issueops.RemoteCompletionSection, audit string) error
	// Processes는 워크트리 점유 관측·종료 표면이고 OrcaTerminals는 워크트리에 매인
	// Orca 터미널 인벤토리·종료 표면이다. 둘 다 nil이면 기본 구현 또는 "Orca 없음"
	// 으로 동작한다(#477).
	Processes     CleanupProcessDeps
	OrcaTerminals port.CleanupOrcaTerminals
	// ObserveArtifact는 원격 artifact의 현재 상태를 provider에서 읽는다.
	// replacement 증거 검증의 유일한 근거이며, 주입되지 않으면 그 경로는 열리지
	// 않는다 — 관측 없이 증거를 인정하지 않는다(#283).
	ObserveArtifact func(url string) (issueopsdomain.ArtifactObservation, error)
}

// cleanupFinishRemedyCommand는 해소 경로가 하나로 정해지는 missing에만 그 명령을
// 돌려준다. 상황에 따라 갈리는 항목(worktree_clean, remote_branch_absent 등)은
// 담지 않는다 — 틀린 안내는 안내가 없는 것보다 나쁘다(이슈 #154).
func cleanupFinishRemedyCommand(id string, missing []string) string {
	if slices.Contains(missing, "completion_reflected") {
		return fmt.Sprintf("issueops remote reflect-completion --id %s --confirm --json", id)
	}
	return ""
}

// keepRemoteBranchFlag는 preview가 발급하는 apply 명령에 caller의 선택을 그대로
// 되돌려준다. 이 플래그가 빠진 명령을 복사해 실행하면 게이트가 다시 막는다.
func keepRemoteBranchFlag(keep bool) string {
	if !keep {
		return ""
	}
	return " --keep-remote-branch"
}

// keptRemoteBranchAudit는 남긴 원격 브랜치를 감사 라인 조각으로 렌더한다.
// 레코드가 삭제되면 이슈 본문의 이 한 줄이 그 브랜치의 유일한 기록이다.
func keptRemoteBranchAudit(kept *issueops.CleanupKeptRemoteBranch) string {
	if kept == nil {
		return ""
	}
	tip := kept.RemoteOID
	if tip == "" {
		tip = kept.State
	}
	return fmt.Sprintf(" remote_branch_kept=%s@%s", kept.Branch, tip)
}

// CleanupFinish는 preview 게이트를 평가하고, apply에서 orca→git 순의 멱등
// 정리 후 레코드를 삭제한다. ②~④ 중 실패하면 레코드를 삭제하지 않고 실패
// 지점을 record에 남긴 채 반환한다(resumable).
func CleanupFinish(ctx context.Context, stateRoot string, req CleanupFinishRequest, deps CleanupFinishDeps) (CleanupFinishResult, error) {
	if deps.Git == nil {
		// remote_branch_absent 게이트가 finish에 첫 네트워크 호출(ls-remote)을
		// 들여온다. preflight.GitCmd에는 비대화·timeout 계약이 없어 자격증명
		// 프롬프트에 걸리면 세션을 붙잡으므로 sync-base가 확립한 계약을 쓴다.
		deps.Git = func(dir string, args ...string) (int, string) {
			return defaultExecutionSyncBaseGit(ctx, dir, args...)
		}
	}
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return CleanupFinishResult{OK: false, ID: req.ID}, err
	}
	inventory, result := (cleanupapp.FinishPreviewer{
		Environment: CleanupFinishEnvironment{RunGit: deps.Git}, ObserveArtifact: deps.ObserveArtifact,
		Workspace: func(ctx context.Context, record issueops.IssueOpsRecord, root string) (cleanupapp.FinishWorkspaceObservation, []string) {
			observed, missing := cleanupWorkspaceGatesForRecord(ctx, record, root, deps.Processes, deps.OrcaTerminals)
			return cleanupapp.FinishWorkspaceObservation{Occupants: observed.Occupants, Receipts: observed.Receipts, Terminals: observed.Terminals, RuntimeReady: observed.RuntimeReady, AppPID: observed.AppPID}, missing
		},
	}).Plan(ctx, record, req)
	missing := result.Missing
	if len(missing) > 0 {
		result.OK = false
		result.NextCommand = cleanupFinishRemedyCommand(record.ID, missing)
		return result, fmt.Errorf("cleanup finish is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := cleanupFinishFingerprint(inventory)
	if err != nil {
		return CleanupFinishResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf("issueops cleanup finish --id %s --apply --confirm --fingerprint %s%s%s --json",
			record.ID, fingerprint, cleanupSupersededByFlag(result.SupersededBy),
			keepRemoteBranchFlag(req.KeepRemoteBranch))
		return result, nil
	}
	if !req.Confirm {
		result.OK = false
		return result, fmt.Errorf("cleanup finish --apply requires --confirm")
	}
	// ① TOCTOU: apply 직전 재계산 일치. 부분 정리·외부 변경이 있었다면 여기서
	// 멈추고 preview 재발급을 요구한다.
	if req.Fingerprint != fingerprint {
		result.OK = false
		return result, fmt.Errorf("stale cleanup fingerprint; run --preview again and retry with the new value")
	}
	// C2-F1: 파괴 단계에 들어가기 전에 보존 payload를 스냅샷한다. ④'는 이
	// 스냅샷으로만 렌더하므로 워크트리 삭제 이후에도 보존 본문이 유지된다.
	completionSnapshot := completionapp.NewCompletionCollector(CompletionArtifacts{}).Collect(record)
	fail := func(step string, stepErr error) (CleanupFinishResult, error) {
		result.OK = false
		result.FailedStep = step
		recordCleanupFinishFailure(stateRoot, record.ID, step, stepErr)
		result.NextCommand = fmt.Sprintf("issueops cleanup finish --id %s --preview --json", record.ID)
		return result, fmt.Errorf("cleanup finish step %s failed (record preserved; re-run preview then apply): %w", step, stepErr)
	}
	// ①′ 워크트리 점유 프로세스·Orca 터미널 종료. 재관측으로 점유 0을 증명하지
	// 못하면 아무것도 지우지 않고 멈춘다(#477).
	if inventory.WorktreePresent && (len(result.WorkspaceProcesses) > 0 || len(inventory.OrcaTerminals) > 0 || inventory.OrcaRuntimeReady) {
		stopped, terminals, err := cleanupStopWorkspace(ctx, inventory.WorktreeRoot, result.WorkspaceProcesses, inventory.OrcaTerminals, inventory.OrcaRuntimeReady, inventory.OrcaAppPID, deps.Processes, deps.OrcaTerminals)
		result.WorkspaceProcessesStopped = stopped
		result.OrcaTerminalsStopped = terminals
		if err != nil {
			return fail(issueops.CleanupFailureStepWorkspaceProcessesStop, err)
		}
	}
	// ② orca 회수 먼저(인벤토리 정합), force=false.
	if inventory.OrcaWorktreeID != "" {
		if deps.RemoveOrcaWorktree == nil {
			return fail(issueops.CleanupFailureStepOrcaRemove, fmt.Errorf("orca worktree remover is not configured"))
		}
		if err := deps.RemoveOrcaWorktree(ctx, inventory.OrcaWorktreeID); err != nil {
			return fail(issueops.CleanupFailureStepOrcaRemove, err)
		}
		result.OrcaRemoved = true
	}
	// ③ git worktree 제거(부재 = 성공). Orca는 정상 제거 시 연결된 Git
	// worktree까지 함께 없애므로, 성공 직후 경로를 다시 관측해 이중 삭제를
	// 피한다. 직접 정리와 경합해 명령 도중 경로가 사라진 경우도 멱등 성공이다.
	worktreePresent := inventory.WorktreePresent
	if worktreePresent && result.OrcaRemoved {
		if _, err := os.Lstat(inventory.WorktreeRoot); os.IsNotExist(err) {
			worktreePresent = false
			result.WorktreeRemoved = true
		}
	}
	if worktreePresent {
		if code, out := deps.Git(record.Repo, "worktree", "remove", inventory.WorktreeRoot); code != 0 {
			if _, err := os.Lstat(inventory.WorktreeRoot); !os.IsNotExist(err) {
				return fail(issueops.CleanupFailureStepWorktreeRemove, fmt.Errorf("git worktree remove: %s", out))
			}
		}
		result.WorktreeRemoved = true
	}
	// ④ 로컬 브랜치 삭제(head OID CAS, 부재 = 생략).
	if inventory.BranchOID != "" {
		if code, out := deps.Git(record.Repo, "update-ref", "-d", "refs/heads/"+inventory.Branch, inventory.BranchOID); code != 0 {
			// ③이 Orca/Git worktree를 제거하면서 linked branch ref까지 함께
			// 회수하는 순서가 있다. 그때 이 단계의 대상은 이미 없고, 부재는
			// 삭제가 목표로 하던 상태 그 자체다. 실패로 처리하면 첫 apply가
			// exact-once/idempotent 계약을 깨고 두 번 실행해야 수렴한다(#291).
			//
			// 판정 근거는 오류 문자열이 아니라 **재관측**이다. Git 메시지는
			// 버전·로케일에 따라 달라지지만 ref 존재 여부는 그렇지 않다.
			// permission, lock contention, OID drift는 ref가 남아 있으므로
			// 그대로 실패한다.
			if branchRefPresent(deps.Git, record.Repo, inventory.Branch) {
				return fail(issueops.CleanupFailureStepBranchDelete, fmt.Errorf("git update-ref -d: %s", out))
			}
		}
		result.BranchDeleted = true
	}
	// ④' 감사 라인 best-effort 멱등 반영 — 실패해도 ⑤를 막지 않는다.
	if deps.ReflectAudit != nil {
		audit := fmt.Sprintf("cleanup 완료: worktree=%s branch=%s oid=%s stopped=%d terminals=%d%s at=%s",
			orNone(inventory.WorktreeRoot), orNone(inventory.Branch), orNone(inventory.BranchOID),
			len(result.WorkspaceProcessesStopped), result.OrcaTerminalsStopped,
			keptRemoteBranchAudit(result.KeptRemoteBranch),
			time.Now().UTC().Format(time.RFC3339))
		if err := deps.ReflectAudit(record, completionSnapshot, audit); err == nil {
			result.AuditReflected = true
		} else {
			// best-effort지만 무흔적 실패는 금지 — 결과에 표면화한다.
			result.AuditError = err.Error()
		}
	}
	// ⑤ 레코드 삭제 — 결정적 ID 재사용과의 충돌을 끝내는 수명 종료.
	if err := deleteIssueOps(stateRoot, record.ID); err != nil {
		return fail(issueops.CleanupFailureStepRecordDelete, err)
	}
	result.RecordDeleted = true
	return result, nil
}

func cleanupFinishFingerprint(inventory issueops.CleanupFinishInventory) (string, error) {
	data, err := json.Marshal(inventory)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// recordCleanupFinishFailure는 실패 지점을 record에 남긴다(resumable 계약).
// 기록 실패는 원 실패 보고를 막지 않는 best-effort다.
func recordCleanupFinishFailure(stateRoot, id, step string, stepErr error) {
	_ = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		rec, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Format(time.RFC3339Nano)
		rec.CleanupFinishFailure = &issueops.IssueOpsCleanupFinishFailure{Step: step, Message: stepErr.Error(), At: now}
		rec.UpdatedAt = now
		_, err = writeIssueOps(stateRoot, rec)
		return err
	})
}

func orNone(v string) string {
	if strings.TrimSpace(v) == "" {
		return "(없음)"
	}
	return v
}

// branchRefPresent는 exact local branch ref가 여전히 존재하는지 재관측한다.
//
// `update-ref -d` 실패를 부재로 정규화할지 판정하는 유일한 근거다. Git의 오류
// 문구는 버전과 로케일에 따라 달라지므로 문자열 매칭 대신 ref 자체를 다시
// 읽는다. 관측이 불가능하면(예: Git 호출 자체가 실패) 존재하는 것으로 보아
// fail-closed한다 — 부재를 증명하지 못한 상태에서 성공으로 정규화하면 실제
// 실패를 삼키게 된다.
func branchRefPresent(git func(dir string, args ...string) (int, string), repo, branch string) bool {
	if git == nil || strings.TrimSpace(branch) == "" {
		return true
	}
	code, _ := git(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return code != 1
}
