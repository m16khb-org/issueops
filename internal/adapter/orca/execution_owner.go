package orca

import (
	"context"
	"fmt"
	"strings"

	"issueops/internal/port"
)

func (p *ExecutionProvisioner) InspectOwner(ctx context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	client, ok := p.client.(executionInventoryClient)
	if !ok {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner inventory is unavailable")
	}
	terminals, err := client.listTerminalsInventory(ctx, req.WorktreeID)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	currentRuntime, err := executionOwnerInventoryRuntime(terminals.RuntimeID, req)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	result := port.ExecutionOrcaOwnerInventory{RuntimeID: currentRuntime, TerminalInventoryComplete: terminals.Complete}
	if err := p.inspectOwnerTerminal(ctx, terminals.Rows, currentRuntime, req, &result); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	tasks, err := client.listRunTasksInventory(ctx, req.RunID, "--brief")
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(tasks.RuntimeID, currentRuntime); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	taskFound, err := inspectOwnerTask(tasks.Rows, currentRuntime, req.TaskID, &result)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	dispatch, err := client.showDispatchInventory(ctx, req.TaskID)
	if err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(dispatch.RuntimeID, currentRuntime); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	if dispatch.Dispatch == nil {
		if !taskFound {
			return result, nil
		}
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner dispatch is absent")
	}
	// 거울상의 모순도 같은 ambiguity다. task 행이 완전 목록에서 사라졌는데
	// dispatch 행이 남아 있으면 어느 쪽 인벤토리를 믿어야 하는지 알 수 없다.
	// 예전에는 dispatch 상태가 비종결일 때만 우연히 막혔다 — 상태 어휘를 판정에서
	// 뺀 뒤로는 행의 존재만으로 막는다. 이 검사는 어휘에 의존하지 않는다.
	if !taskFound {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca owner task is absent while its dispatch row remains")
	}
	if err := inspectOwnerDispatchRow(*dispatch.Dispatch, terminals.Rows, currentRuntime, req, &result); err != nil {
		return port.ExecutionOrcaOwnerInventory{}, err
	}
	return result, nil
}

// inspectOwnerTerminal finds the single owner terminal row for the request PTY
// and records its identity and liveness on result.
func (p *ExecutionProvisioner) inspectOwnerTerminal(ctx context.Context, rows []port.OrcaTerminal, currentRuntime string, req port.ExecutionOrcaOwnerInventoryRequest, result *port.ExecutionOrcaOwnerInventory) error {
	for _, terminal := range rows {
		if terminal.PTYID != req.TerminalPTYID {
			continue
		}
		if result.TerminalID != "" {
			return fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		if terminal.RuntimeID != currentRuntime {
			return fmt.Errorf("Orca owner terminal runtime identity changed")
		}
		result.TerminalID = terminal.PTYID
		live, err := p.inspectOwnerTerminalDetail(ctx, terminal, currentRuntime, req.WorktreeID)
		if err != nil {
			return err
		}
		result.TerminalLive = live
	}
	return nil
}

// inspectOwnerTerminalDetail verifies terminal show output against the listed
// row and reports whether the terminal still has a live pane.
func (p *ExecutionProvisioner) inspectOwnerTerminalDetail(ctx context.Context, terminal port.OrcaTerminal, currentRuntime, worktreeID string) (bool, error) {
	detailClient, ok := p.client.(executionTerminalDetailClient)
	if !ok {
		return false, fmt.Errorf("Orca owner terminal detail inventory is unavailable")
	}
	detail, err := detailClient.showTerminalInventory(ctx, terminal.Handle)
	if err != nil {
		return false, err
	}
	if err := validateExecutionInventoryRuntime(detail.RuntimeID, currentRuntime); err != nil {
		return false, err
	}
	if detail.Terminal.RuntimeID != currentRuntime ||
		detail.Terminal.Handle != terminal.Handle ||
		detail.Terminal.PTYID != terminal.PTYID ||
		detail.Terminal.WorktreeID != worktreeID {
		return false, fmt.Errorf("Orca owner terminal detail identity changed")
	}
	if detail.PaneRuntimeID == nil {
		return false, fmt.Errorf("Orca owner terminal pane runtime identity is unavailable")
	}
	// Orca 재시작 뒤 terminal list에는 connected/writable인 장부 행이 남을 수 있다.
	// terminal show의 음수 paneRuntimeId는 현재 렌더러에 실제 pane이 없다는 증거다.
	return detail.Terminal.Connected && detail.Terminal.Writable && *detail.PaneRuntimeID >= 0, nil
}

// inspectOwnerTask locates the single task row for taskID, records its status
// on result, and reports whether the row was found.
func inspectOwnerTask(rows []port.OrcaTask, currentRuntime, taskID string, result *port.ExecutionOrcaOwnerInventory) (bool, error) {
	taskFound := false
	for _, task := range rows {
		if task.ID != taskID {
			continue
		}
		if taskFound {
			return false, fmt.Errorf("Orca owner task inventory is ambiguous")
		}
		taskFound = true
		if task.RuntimeID != currentRuntime {
			return false, fmt.Errorf("Orca owner task runtime identity changed")
		}
		result.TaskStatus = strings.ToLower(strings.TrimSpace(task.Status))
		result.TaskLive = !executionTerminalTaskStatus(result.TaskStatus)
	}
	return taskFound, nil
}

// inspectOwnerDispatchRow verifies the dispatch row identity and records its
// assignee presence and status on result.
func inspectOwnerDispatchRow(row port.OrcaDispatch, terminals []port.OrcaTerminal, currentRuntime string, req port.ExecutionOrcaOwnerInventoryRequest, result *port.ExecutionOrcaOwnerInventory) error {
	if row.RuntimeID != currentRuntime || row.ID != req.DispatchID || row.TaskID != req.TaskID {
		return fmt.Errorf("Orca owner dispatch identity changed")
	}
	result.DispatchAssigneeHandle = strings.TrimSpace(row.AssigneeHandle)
	for _, terminal := range terminals {
		if terminal.Handle != result.DispatchAssigneeHandle {
			continue
		}
		if terminal.RuntimeID != currentRuntime || result.DispatchAssigneePresent {
			return fmt.Errorf("Orca owner dispatch assignee inventory is ambiguous")
		}
		result.DispatchAssigneePresent = true
	}
	result.DispatchStatus = strings.ToLower(strings.TrimSpace(row.Status))
	return nil
}

func executionOwnerInventoryRuntime(observed string, req port.ExecutionOrcaOwnerInventoryRequest) (string, error) {
	observed = strings.TrimSpace(observed)
	sealed := strings.TrimSpace(req.RuntimeID)
	if observed == "" || sealed == "" || observed != sealed && !req.AllowRuntimeRollover {
		return "", fmt.Errorf("Orca inventory runtime identity changed")
	}
	return observed, nil
}

// executionTerminalTaskStatus reports whether a status means the work is over.
//
// The vocabulary comes from the Orca CLI, not from another Go definition:
//
//	$ orca orchestration task-update --help
//	Notes:
//	  Valid --status values: pending, ready, dispatched, completed, failed, blocked.
//
// Only completed and failed are terminal. The other four can still hold or
// acquire a worker. This set used to also list complete/cancelled/canceled/
// closed, which Orca rejects outright — carrying values that cannot be observed
// obscured where the vocabulary comes from without defending against anything,
// since an unrecognised status already falls through to "not terminal" (#145).
//
// core/operationalhealth mirrors this set in settledTaskStatus. The two must
// agree, and the CLI decides on what.
func executionTerminalTaskStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "failed":
		return true
	default:
		return false
	}
}
