package issueopscleanup

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// WorkspaceCleaner coordinates occupancy and terminal observations with cleanup.
type WorkspaceCleaner struct {
	Processes       WorkspaceProcessControl
	Orca            port.CleanupOrcaTerminals
	SamePath        func(string, string) (bool, error)
	PaneKey, Handle string
}

// Observe는 finish/abandon이 공유하는 워크트리 게이트다.
// 소스 체크아웃은 관측 전에 거부하고, Orca 바인딩 여부를 레코드에서 읽는다.
func (s WorkspaceCleaner) Observe(ctx context.Context, record issueops.IssueOpsRecord, root string) (FinishWorkspaceObservation, []string) {
	sourceCheckout, pathErr := s.SamePath(root, record.Repo)
	if pathErr != nil || sourceCheckout {
		return FinishWorkspaceObservation{}, []string{"worktree_is_source_checkout"}
	}
	bound := domain.CleanupWorkspaceOrcaBound(record)
	return s.observe(ctx, root, bound)
}

// observe는 점유 관측·요청자 게이트·Orca 터미널 인벤토리를
// 평가한다(#477). 점유 자체는 게이트를 막지 않는다 — apply ①′의 종료 대상이다.
//
// 요청자 보호는 제외가 아니라 거부다. lease 경로는 요청자 자손을 quiescence
// 후보에서 *제외*하지만(직접 승계가 성립해야 하므로), cleanup은 워크트리를
// 지우므로 요청자가 그 안에 서 있으면 진행 자체가 잘못이다.
func (s WorkspaceCleaner) observe(ctx context.Context, root string, bound bool) (FinishWorkspaceObservation, []string) {
	occupancy, err := s.Processes.Observe(root)
	if err != nil {
		return FinishWorkspaceObservation{}, []string{"workspace_processes_observable"}
	}
	missing, ok := domain.CleanupRequesterOccupancyMissing(occupancy.Occupants, occupancy.Ancestry, s.Processes.RequesterPID())
	if !ok {
		return FinishWorkspaceObservation{}, []string{"workspace_processes_observable"}
	}
	observation := FinishWorkspaceObservation{Occupants: occupancy.Occupants, Receipts: domain.CleanupOccupantReceipts(occupancy.Occupants)}
	requester := cleanupRequesterEnv{
		paneKey: strings.TrimSpace(s.PaneKey),
		handle:  strings.TrimSpace(s.Handle),
	}
	orcaMissing := s.orcaGates(ctx, root, bound, requester, &observation)
	return observation, append(missing, orcaMissing...)
}

// cleanupRequesterEnv는 요청자 터미널을 확정하는 env다. 무선택자
// `orca terminal show`는 호출자가 아니라 UI-active 터미널을 돌려주므로(2026-08-27
// 실측) env를 전체 터미널 인벤토리와 join해서만 요청자 행을 찾는다.
type cleanupRequesterEnv struct {
	paneKey string
	handle  string
}

func (env cleanupRequesterEnv) hosted() bool { return env.paneKey != "" || env.handle != "" }

// orcaGates는 Orca 런타임 상태·요청자 터미널·워크트리 터미널 인벤토리를
// 평가하고 observation의 Terminals/RuntimeReady/AppPID를 채운다.
func (s WorkspaceCleaner) orcaGates(ctx context.Context, root string, bound bool, requester cleanupRequesterEnv, observation *FinishWorkspaceObservation) []string {
	missing := []string{}
	if s.Orca == nil {
		// Orca 표면이 배선되지 않으면 apply도 exact terminal close를 부르지 않으므로
		// 요청자 터미널 게이트가 막아야 할 터미널 단위 종료가 없다. 요청자 보호는
		// pid-조상 게이트로 남고, Orca 바인딩 사이클은 여전히 런타임을 요구한다.
		if bound {
			missing = append(missing, "orca_runtime_ready")
		}
		return missing
	}
	// 런타임이 ready면 점유·바인딩·호스팅과 무관하게 워크트리 터미널을 나열한다.
	// cwd를 옮긴 셸은 점유자가 아니어도 Orca 레지스트리에는 워크트리 터미널로
	// 남아 있고, apply ①′가 닫아야 한다(AC-01).
	status, err := s.Orca.Status(ctx)
	runtimeReady := err == nil && status.RuntimeReachable && status.RuntimeState == "ready"
	ready := runtimeReady && (!bound || status.GraphState == "ready")
	observation.RuntimeReady = ready
	observation.AppPID = status.AppPID
	if requester.hosted() {
		missing = append(missing, s.requesterTerminalMissing(ctx, root, requester, ready)...)
	}
	switch {
	case ready:
		rows, listErr := s.Orca.ListWorktreeTerminalsByPath(ctx, root)
		if listErr != nil {
			missing = append(missing, "orca_terminals_observable")
		} else {
			handles, handlesErr := s.terminalHandles(root, rows)
			if handlesErr != nil {
				missing = append(missing, "orca_terminals_observable")
			} else {
				observation.Terminals = handles
			}
		}
	case bound:
		// Orca 바인딩 사이클은 런타임 없이 터미널을 죽이면 ②(orca 회수)가 확실히
		// 실패하는 파괴적 부분 apply가 된다(design-review 2차 finding 11).
		missing = append(missing, "orca_runtime_ready")
	}
	return missing
}

func (s WorkspaceCleaner) requesterTerminalMissing(ctx context.Context, root string, requester cleanupRequesterEnv, ready bool) []string {
	if !ready {
		return []string{"requester_terminal_unresolved"}
	}
	rows, err := s.Orca.ListAllTerminals(ctx)
	if err != nil {
		return []string{"requester_terminal_unresolved"}
	}
	row, found := domain.CleanupRequesterTerminal(cleanupTerminalIdentities(rows), requester.paneKey, requester.handle)
	worktreePath := strings.TrimSpace(row.WorktreePath)
	switch {
	case !found:
		return []string{"requester_terminal_unresolved"}
	case worktreePath == "":
		return []string{"requester_terminal_unresolved"}
	}
	same, err := s.SamePath(worktreePath, root)
	switch {
	case err != nil:
		return []string{"requester_terminal_unresolved"}
	case same:
		return []string{"requester_terminal_outside_worktree"}
	}
	return nil
}

func (s WorkspaceCleaner) terminalHandles(root string, rows []port.OrcaTerminal) ([]string, error) {
	handles := make([]string, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		handle := strings.TrimSpace(row.Handle)
		worktreePath := strings.TrimSpace(row.WorktreePath)
		sameWorktree, pathErr := s.SamePath(worktreePath, root)
		if handle == "" || pathErr != nil || !sameWorktree || seen[handle] {
			return nil, fmt.Errorf("malformed Orca terminal inventory")
		}
		seen[handle] = true
		handles = append(handles, handle)
	}
	sort.Strings(handles)
	return handles, nil
}

// Stop은 apply ①′다: preview가 나열한 Orca 터미널이 있으면
// exact handle별 `orca terminal close`를 먼저 부르고, 남은 점유자를 receipt 결속
// 아래 HUP/TERM/KILL로 종료한다. 터미널 stop 실패는 fail-closed다 — 시그널
// 경로로 넘어가면 터미널은 죽고 Orca 회수는 실패하는 파괴적 부분 apply가 된다.
// Orca 상태(ready, app pid)는 apply 직전 게이트 재평가가 fingerprint로 고정했으므로
// 여기서 다시 묻지 않는다; 런타임이 사라졌다면 fingerprint가 이미 어긋난다.
func (s WorkspaceCleaner) Stop(ctx context.Context, root string, occupants []issueops.CleanupWorkspaceProcess, terminals []string, orcaRuntimeReady bool, appPID int) ([]issueops.CleanupWorkspaceProcess, int, error) {
	excluded := map[int]bool{}
	if appPID > 0 {
		excluded[appPID] = true
	}
	terminalsStopped := 0
	if len(terminals) > 0 {
		if s.Orca == nil {
			return nil, 0, fmt.Errorf("orca terminals %v were previewed but no orca surface is wired", terminals)
		}
		for _, handle := range terminals {
			if err := s.Orca.CloseTerminal(ctx, handle); err != nil {
				return nil, terminalsStopped, fmt.Errorf("close Orca terminal %s for %s: %w", handle, root, err)
			}
			terminalsStopped++
		}
		if err := s.requireNoTerminals(ctx, root); err != nil {
			return nil, terminalsStopped, err
		}
	}
	stopped, err := (WorkspaceProcessStopper{Processes: s.Processes}).Stop(root, occupants, excluded)
	if err != nil {
		return stopped, terminalsStopped, err
	}
	if orcaRuntimeReady {
		if s.Orca == nil {
			return stopped, terminalsStopped, fmt.Errorf("Orca runtime was previewed ready but no Orca surface is wired")
		}
		if err := s.requireNoTerminals(ctx, root); err != nil {
			return stopped, terminalsStopped, err
		}
	}
	return stopped, terminalsStopped, nil
}

func (s WorkspaceCleaner) requireNoTerminals(ctx context.Context, root string) error {
	remaining, err := s.Orca.ListWorktreeTerminalsByPath(ctx, root)
	if err != nil {
		return fmt.Errorf("observe Orca terminals before deletion for %s: %w", root, err)
	}
	handles, err := s.terminalHandles(root, remaining)
	if err != nil {
		return fmt.Errorf("observe Orca terminals before deletion for %s: %w", root, err)
	}
	if len(handles) > 0 {
		return fmt.Errorf("Orca terminals remain before deletion for %s: %v", root, handles)
	}
	return nil
}

func cleanupTerminalIdentities(rows []port.OrcaTerminal) []domain.CleanupTerminalIdentity {
	identities := make([]domain.CleanupTerminalIdentity, len(rows))
	for i, row := range rows {
		identities[i] = domain.CleanupTerminalIdentity{Handle: row.Handle, TabID: row.TabID, LeafID: row.LeafID, WorktreePath: row.WorktreePath}
	}
	return identities
}
