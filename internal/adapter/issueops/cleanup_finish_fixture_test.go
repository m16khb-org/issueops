package issueops

import (
	"context"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	completionapp "issueops/internal/application/issueopsremote"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

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

// CleanupFinish is only a fixture for existing integration scenarios. Production
// composition calls the application executor directly.
func CleanupFinish(ctx context.Context, stateRoot string, req CleanupFinishRequest, deps CleanupFinishDeps) (CleanupFinishResult, error) {
	return finishExecutorForTests(stateRoot, deps).Run(ctx, req)
}

func finishExecutorForTests(stateRoot string, deps CleanupFinishDeps) cleanupapp.FinishExecutor {
	runtime := CleanupFinishRuntime{RunGit: deps.Git, Processes: deps.Processes, OrcaTerminals: deps.OrcaTerminals}
	executor := cleanupapp.FinishExecutor{
		Records: CleanupRecordStore{StateRoot: stateRoot}, Acquire: (CleanupLifetimeLock{StateRoot: stateRoot}).Acquire,
		Observe: func(_ context.Context, _ issueops.IssueOpsRecord, r issueops.CleanupFinishRequest) (issueops.CleanupFinishRequest, error) {
			return r, nil
		},
		Plan: func(ctx context.Context, record issueops.IssueOpsRecord, req issueops.CleanupFinishRequest) (issueops.CleanupFinishInventory, issueops.CleanupFinishResult) {
			return (cleanupapp.FinishPreviewer{Environment: CleanupFinishEnvironment{RunGit: func(dir string, args ...string) (int, string) { return runtime.Git(ctx, dir, args...) }}, ObserveArtifact: deps.ObserveArtifact, Workspace: runtime.Workspace}).Plan(ctx, record, req)
		},
		Fingerprint: CleanupFinishFingerprint, NewAttempt: NewCleanupAttempt,
		Completion: completionapp.NewCompletionCollector(CompletionArtifacts{}).Collect,
		Stop:       runtime.Stop, RemoveOrca: deps.RemoveOrcaWorktree, Directory: (CleanupFinishEnvironment{}).Directory, Git: runtime.Git, Now: time.Now,
	}
	if deps.ReflectAudit != nil {
		executor.ReflectAudit = func(_ context.Context, record issueops.IssueOpsRecord, completion issueops.RemoteCompletionSection, audit string) error {
			return deps.ReflectAudit(record, completion, audit)
		}
	}
	return executor
}
