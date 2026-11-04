package issueops

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	model "issueops/internal/contract/issueops"
)

// ErrCleanupOccupancyChanged 변수는 apply 시점의 워크스페이스 점유자가 preview receipt와 달라졌음을
// 나타내는 오류다. 호출자는 errors.Is로 확인하고 preview를 다시 실행해야 한다.
var ErrCleanupOccupancyChanged = errors.New("workspace occupancy changed since preview")

func cleanupReceiptMatches(left, right model.CleanupWorkspaceProcess) bool {
	return left.PID == right.PID && left.StartedAt == right.StartedAt && left.Executable == right.Executable
}

// CleanupStopTargets 함수는 apply 시점 점유자 가운데 preview receipt에 결속된 것만 신호 대상으로
// 확정한다. 요청자 자신·조상 pid가 점유 중이면 거부다.
func CleanupStopTargets(occupants []model.CleanupWorkspaceProcess, ancestry map[int][]int, selfPID int, preview []model.CleanupWorkspaceProcess) ([]model.CleanupWorkspaceProcess, error) {
	selfAncestry, ok := ancestry[selfPID]
	if !ok {
		return nil, fmt.Errorf("requester process %d ancestry is unobservable", selfPID)
	}
	protected := map[int]bool{selfPID: true}
	for _, pid := range selfAncestry {
		protected[pid] = true
	}
	previewByPID := map[int]model.CleanupWorkspaceProcess{}
	for _, item := range preview {
		previewByPID[item.PID] = item
	}
	currentByPID := map[int]model.CleanupWorkspaceProcess{}
	for _, occupant := range occupants {
		currentByPID[occupant.PID] = occupant
	}
	targets := make([]model.CleanupWorkspaceProcess, 0, len(occupants))
	for _, occupant := range occupants {
		if protected[occupant.PID] {
			return nil, fmt.Errorf("requester process %d (%s) occupies the worktree; cleanup refuses to signal its own session", occupant.PID, occupant.Command)
		}
		if !cleanupOccupantBound(occupant, previewByPID, currentByPID, ancestry[occupant.PID]) {
			return nil, fmt.Errorf("%w: pid=%d command=%s started_at=%s", ErrCleanupOccupancyChanged, occupant.PID, occupant.Command, occupant.StartedAt)
		}
		targets = append(targets, occupant)
	}
	return targets, nil
}

// cleanupOccupantBound는 점유자가 preview receipt와 같거나, preview 점유자의
// 증명된 자손이면서 그 조상이 지금도 같은 receipt로 점유 중일 때 참이다. 자손
// 관계는 stale 허용 근거일 뿐 종료 범위 확장이 아니다 — 점유하지 않는 자손에는
// 신호를 보내지 않는다(architecture-review 2차 finding 2).
func cleanupOccupantBound(occupant model.CleanupWorkspaceProcess, previewByPID, currentByPID map[int]model.CleanupWorkspaceProcess, ancestors []int) bool {
	if previewed, ok := previewByPID[occupant.PID]; ok && cleanupReceiptMatches(previewed, occupant) {
		return true
	}
	for _, ancestor := range ancestors {
		previewed, ok := previewByPID[ancestor]
		if !ok {
			continue
		}
		if current, live := currentByPID[ancestor]; live && cleanupReceiptMatches(previewed, current) {
			return true
		}
	}
	return false
}

// DescribeCleanupProcesses 함수는 프로세스 목록을 `pid:command`를 쉼표로 이은 한 줄로 만든다. 비어
// 있으면 "(none)"이다.
func DescribeCleanupProcesses(processes []model.CleanupWorkspaceProcess) string {
	if len(processes) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(processes))
	for _, process := range processes {
		parts = append(parts, fmt.Sprintf("%d:%s", process.PID, process.Command))
	}
	return strings.Join(parts, ", ")
}

// RemainingCleanupProcesses 함수는 종료 대상 targets 중 여전히 같은 PID·시작 시각·실행 파일로
// occupants에 남아 있는 것을 돌려준다.
func RemainingCleanupProcesses(targets, occupants []model.CleanupWorkspaceProcess) []model.CleanupWorkspaceProcess {
	current := make(map[int]model.CleanupWorkspaceProcess, len(occupants))
	for _, occupant := range occupants {
		current[occupant.PID] = occupant
	}
	remaining := make([]model.CleanupWorkspaceProcess, 0, len(targets))
	for _, target := range targets {
		if occupant, ok := current[target.PID]; ok && cleanupReceiptMatches(occupant, target) {
			remaining = append(remaining, target)
		}
	}
	return remaining
}

// CleanupRequesterOccupancyMissing 함수는 요청자 프로세스나 그 조상이 워크트리를 점유하는지 판정한다. 점유
// 중이면 `requester_occupies_worktree`를 돌려주고, 요청자 조상을 관측하지 못하면 두 번째 반환값이 false다.
func CleanupRequesterOccupancyMissing(occupants []model.CleanupWorkspaceProcess, ancestry map[int][]int, selfPID int) ([]string, bool) {
	selfAncestry, ok := ancestry[selfPID]
	if !ok {
		return nil, false
	}
	protected := map[int]bool{selfPID: true}
	for _, pid := range selfAncestry {
		protected[pid] = true
	}
	for _, occupant := range occupants {
		if protected[occupant.PID] {
			return []string{"requester_occupies_worktree"}, true
		}
	}
	return []string{}, true
}

// CleanupOccupantReceipts 함수는 점유 프로세스를 PID 순으로 정렬한 receipt(PID, 시작 시각, 실행
// 파일)로 바꾼다.
func CleanupOccupantReceipts(occupants []model.CleanupWorkspaceProcess) []model.NativeProcessReceipt {
	receipts := make([]model.NativeProcessReceipt, 0, len(occupants))
	for _, occupant := range occupants {
		receipts = append(receipts, model.NativeProcessReceipt{PID: occupant.PID, StartedAt: occupant.StartedAt, Executable: occupant.Executable})
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].PID < receipts[j].PID })
	return receipts
}
