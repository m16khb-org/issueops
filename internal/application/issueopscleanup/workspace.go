package issueopscleanup

import (
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

// WorkspaceCleaner는 워크트리 점유 관측과 점유 프로세스 종료를 cleanup에 연결한다.
type WorkspaceCleaner struct {
	Processes WorkspaceProcessControl
	SamePath  func(string, string) (bool, error)
}

// Observe는 finish/abandon이 공유하는 워크트리 게이트다.
// 소스 체크아웃은 관측 전에 거부한다. 점유 자체는 게이트를 막지 않는다 — apply ①′의
// 종료 대상이다(#477).
//
// 요청자 보호는 제외가 아니라 거부다. lease 경로는 요청자 자손을 quiescence
// 후보에서 *제외*하지만(직접 승계가 성립해야 하므로), cleanup은 워크트리를
// 지우므로 요청자가 그 안에 서 있으면 진행 자체가 잘못이다.
func (s WorkspaceCleaner) Observe(root, repo string) (FinishWorkspaceObservation, []string) {
	sourceCheckout, pathErr := s.SamePath(root, repo)
	if pathErr != nil || sourceCheckout {
		return FinishWorkspaceObservation{}, []string{"worktree_is_source_checkout"}
	}
	occupancy, err := s.Processes.Observe(root)
	if err != nil {
		return FinishWorkspaceObservation{}, []string{"workspace_processes_observable"}
	}
	missing, ok := domain.CleanupRequesterOccupancyMissing(occupancy.Occupants, occupancy.Ancestry, s.Processes.RequesterPID())
	if !ok {
		return FinishWorkspaceObservation{}, []string{"workspace_processes_observable"}
	}
	return FinishWorkspaceObservation{Occupants: occupancy.Occupants, Receipts: domain.CleanupOccupantReceipts(occupancy.Occupants)}, missing
}

// Stop은 apply ①′다: 남은 점유자를 receipt 결속 아래 HUP/TERM/KILL로 종료한다.
func (s WorkspaceCleaner) Stop(root string, occupants []issueops.CleanupWorkspaceProcess) ([]issueops.CleanupWorkspaceProcess, error) {
	return (WorkspaceProcessStopper{Processes: s.Processes}).Stop(root, occupants)
}
