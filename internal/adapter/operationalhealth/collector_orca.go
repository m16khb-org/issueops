package operationalhealth

import (
	"context"
	"strings"

	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	corehealth "issueops/internal/domain/operationalhealth"
	"issueops/internal/port"

	"golang.org/x/sync/errgroup"
)

func (collector Collector) collectOrca(ctx context.Context, snapshot *corehealth.Snapshot, owned bool) {
	if collector.Orca == nil || !collector.Orca.Available() {
		if owned {
			addProblem(snapshot, "orca", "orca_unavailable", "Orca-owned IssueOps records exist but Orca is unavailable")
		} else {
			snapshot.Messages.CompleteAbsence = true
		}
		return
	}
	status, err := collector.Orca.Status(ctx)
	if err != nil {
		recordOrcaUnobserved(snapshot, owned, "orca_status", "orca_status_failed", "Orca status inventory failed")
		return
	}
	runtimeID := strings.TrimSpace(status.RuntimeID)
	if !orcaStatusReady(status, runtimeID) {
		recordOrcaUnobserved(snapshot, owned, "orca_status", "orca_runtime_unready", "Orca runtime and graph must be reachable and ready")
		return
	}
	snapshot.OrcaObserved = true
	snapshot.OrcaRuntimeID = runtimeID

	resolved, repoPath := collector.collectOrcaRepo(ctx, snapshot)
	collector.collectOrcaWorktrees(ctx, snapshot, resolved, repoPath)
	collector.collectOrcaTerminals(ctx, snapshot)

	runs, err := collector.Orca.ListRunInventory(ctx)
	if err != nil {
		addProblem(snapshot, "orca_runs", "orca_runs_failed", "Orca Run inventory failed")
		return
	}
	if !validateOrcaRunInventory(snapshot, runs) {
		return
	}

	readers := collector.readOrcaRunInventories(ctx, runs)
	collector.applyOrcaTasks(snapshot, readers)
	collector.applyOrcaDispatches(ctx, snapshot, readers)
	applyOrcaGates(snapshot, readers)
	collector.collectOrcaInbox(ctx, snapshot)
}

// recordOrcaUnobserved reports an Orca inventory failure only when Orca-owned
// records exist; otherwise it marks the message inventory as completely absent.
func recordOrcaUnobserved(snapshot *corehealth.Snapshot, owned bool, source, code, detail string) {
	if owned {
		addProblem(snapshot, source, code, detail)
		return
	}
	snapshot.Messages.CompleteAbsence = true
}

func orcaStatusReady(status port.OrcaStatus, runtimeID string) bool {
	return runtimeID != "" && status.RuntimeReachable && strings.TrimSpace(status.RuntimeState) == "ready" && strings.TrimSpace(status.GraphState) == "ready"
}

func (collector Collector) collectOrcaRepo(ctx context.Context, snapshot *corehealth.Snapshot) (port.OrcaRepo, string) {
	resolved, resolveErr := collector.Orca.ResolveRepo(ctx, snapshot.RepoRoot)
	if resolveErr != nil {
		addProblem(snapshot, "orca_repo", "orca_repo_failed", "Orca repo resolution failed")
	} else if strings.TrimSpace(resolved.RuntimeID) != snapshot.OrcaRuntimeID || strings.TrimSpace(resolved.ID) == "" || canonicalInventoryPath(resolved.Path) != snapshot.RepoRoot {
		addProblem(snapshot, "orca_repo", "orca_repo_identity_mismatch", "Orca repo identity does not match the requested repository")
	}
	snapshot.OrcaRepoID = strings.TrimSpace(resolved.ID)
	repoPath := snapshot.RepoRoot
	if resolveErr == nil && canonicalInventoryPath(resolved.Path) != "" {
		repoPath = canonicalInventoryPath(resolved.Path)
	}
	return resolved, repoPath
}

func (collector Collector) collectOrcaWorktrees(ctx context.Context, snapshot *corehealth.Snapshot, resolved port.OrcaRepo, repoPath string) {
	worktrees, err := collector.Orca.ListWorktrees(ctx, snapshot.RepoRoot)
	if err != nil {
		addProblem(snapshot, "orca_worktrees", "orca_worktrees_failed", "Orca worktree inventory failed")
		return
	}
	for _, value := range worktrees {
		if orcaWorktreeIdentityInvalid(value, resolved, snapshot.OrcaRuntimeID) {
			addProblem(snapshot, "orca_worktrees", "orca_worktree_identity_invalid", "Orca worktree identity is incomplete or mismatched")
		}
		snapshot.OrcaWorktrees = append(snapshot.OrcaWorktrees, operationalhealthcontract.OrcaWorktree{
			RuntimeID: value.RuntimeID, RepoID: value.RepoID, ID: value.ID, InstanceID: value.InstanceID, Repo: repoPath,
			Path: canonicalInventoryPath(value.Path), Branch: strings.TrimSpace(value.Branch), Head: strings.TrimSpace(value.Head),
		})
	}
}

func orcaWorktreeIdentityInvalid(value port.OrcaWorktree, resolved port.OrcaRepo, runtimeID string) bool {
	return strings.TrimSpace(value.RuntimeID) != runtimeID || strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.InstanceID) == "" || canonicalInventoryPath(value.Path) == "" || (resolved.ID != "" && strings.TrimSpace(value.RepoID) != strings.TrimSpace(resolved.ID))
}

func (collector Collector) collectOrcaTerminals(ctx context.Context, snapshot *corehealth.Snapshot) {
	terminals, err := collector.Orca.ListTerminals(ctx, "")
	if err != nil {
		addProblem(snapshot, "orca_terminals", "orca_terminals_failed", "Orca terminal inventory failed")
		return
	}
	for _, value := range terminals {
		if orcaTerminalIdentityInvalid(value, snapshot.OrcaRuntimeID) {
			addProblem(snapshot, "orca_terminals", "orca_terminal_identity_invalid", "Orca terminal identity is incomplete")
		}
		snapshot.Terminals = append(snapshot.Terminals, operationalhealthcontract.OrcaTerminal{
			RuntimeID: value.RuntimeID, Handle: value.Handle, PTYID: value.PTYID, TabID: value.TabID, LeafID: value.LeafID, WorktreeID: value.WorktreeID,
			WorktreePath: canonicalInventoryPath(value.WorktreePath), Connected: value.Connected, Writable: value.Writable,
		})
	}
}

func orcaTerminalIdentityInvalid(value port.OrcaTerminal, runtimeID string) bool {
	return strings.TrimSpace(value.RuntimeID) != runtimeID || strings.TrimSpace(value.Handle) == "" || strings.TrimSpace(value.PTYID) == "" || strings.TrimSpace(value.WorktreeID) == "" || strings.TrimSpace(value.TabID) == "" || strings.TrimSpace(value.LeafID) == ""
}

// validateOrcaRunInventory records the first run-identity problem and reports
// whether the snapshot is consistent enough to read tasks, dispatches, and gates.
func validateOrcaRunInventory(snapshot *corehealth.Snapshot, runs port.OrcaRunInventory) bool {
	if strings.TrimSpace(runs.RuntimeID) != snapshot.OrcaRuntimeID {
		addProblem(snapshot, "orca_runs", "orca_run_runtime_mismatch", "Run snapshot runtime identity does not match")
		return false
	}
	for _, run := range runs.Runs {
		if strings.TrimSpace(run.RuntimeID) != snapshot.OrcaRuntimeID || strings.TrimSpace(run.ID) == "" {
			addProblem(snapshot, "orca_runs", "orca_run_identity_mismatch", "Run snapshot identity is incomplete or mismatched")
			return false
		}
	}
	return true
}

type orcaRunReaders struct {
	tasks         []port.OrcaTask
	dispatched    []port.OrcaTask
	gates         []port.OrcaGate
	tasksErr      error
	dispatchedErr error
	gatesErr      error
}

func (collector Collector) readOrcaRunInventories(ctx context.Context, runs port.OrcaRunInventory) *orcaRunReaders {
	result := &orcaRunReaders{}
	readers, readerCtx := errgroup.WithContext(ctx)
	readers.Go(func() error {
		result.tasks, result.tasksErr = collector.Orca.ListAllTasksFromRuns(readerCtx, runs)
		return nil
	})
	readers.Go(func() error {
		result.dispatched, result.dispatchedErr = collector.Orca.ListDispatchedTasksFromRuns(readerCtx, runs)
		return nil
	})
	readers.Go(func() error {
		result.gates, result.gatesErr = collector.Orca.ListGatesFromRuns(readerCtx, runs)
		return nil
	})
	_ = readers.Wait()
	return result
}

func (collector Collector) applyOrcaTasks(snapshot *corehealth.Snapshot, readers *orcaRunReaders) {
	if readers.tasksErr != nil {
		addProblem(snapshot, "orca_tasks", "orca_tasks_failed", "Orca task inventory failed")
		return
	}
	for _, value := range readers.tasks {
		task := operationalhealthcontract.OrcaTask{RuntimeID: strings.TrimSpace(value.RuntimeID), RunID: strings.TrimSpace(value.RunID), ID: strings.TrimSpace(value.ID), Status: strings.TrimSpace(value.Status), HasResult: value.HasResult}
		if task.RuntimeID != snapshot.OrcaRuntimeID {
			addProblem(snapshot, "orca_tasks", "orca_task_runtime_mismatch", "task "+task.ID+" runtime identity does not match")
		}
		if strings.TrimSpace(value.CompletedAt) != "" {
			parsed, parseErr := parseTaskCompletedAt(value.CompletedAt)
			if parseErr != nil {
				addProblem(snapshot, "orca_tasks", "orca_task_timestamp_invalid", "task "+task.ID+" has an invalid completion timestamp")
			} else {
				task.CompletedAt = parsed
			}
		}
		snapshot.Tasks = append(snapshot.Tasks, task)
	}
}

func (collector Collector) applyOrcaDispatches(ctx context.Context, snapshot *corehealth.Snapshot, readers *orcaRunReaders) {
	if readers.dispatchedErr != nil {
		addProblem(snapshot, "orca_dispatches", "orca_dispatched_tasks_failed", "Orca dispatched-task inventory failed")
		return
	}
	dispatched := readers.dispatched
	dispatchedCounts := make(map[string]int, len(dispatched))
	for _, task := range dispatched {
		taskID := strings.TrimSpace(task.ID)
		if strings.TrimSpace(task.RuntimeID) != snapshot.OrcaRuntimeID {
			addProblem(snapshot, "orca_dispatches", "orca_dispatched_task_runtime_mismatch", "dispatched task "+taskID+" runtime identity does not match")
		}
		dispatchedCounts[operationalTaskKey(task.RunID, taskID)]++
	}
	for _, task := range snapshot.Tasks {
		if strings.TrimSpace(task.Status) == "dispatched" && dispatchedCounts[operationalTaskKey(task.RunID, task.ID)] != 1 {
			addProblem(snapshot, "orca_dispatches", "orca_dispatch_task_mismatch", "dispatched task sets do not match")
		}
	}
	for _, task := range dispatched {
		collector.collectOrcaDispatch(ctx, snapshot, task, dispatchedCounts)
	}
}

func (collector Collector) collectOrcaDispatch(ctx context.Context, snapshot *corehealth.Snapshot, task port.OrcaTask, dispatchedCounts map[string]int) {
	taskID := strings.TrimSpace(task.ID)
	taskKey := operationalTaskKey(task.RunID, taskID)
	if orcaDispatchedTaskAmbiguous(snapshot, task, taskID, dispatchedCounts[taskKey]) {
		addProblem(snapshot, "orca_dispatches", "orca_dispatch_task_mismatch", "dispatched task identity is missing or ambiguous")
	}
	if taskID == "" {
		return
	}
	dispatch, showErr := collector.Orca.ShowDispatch(ctx, taskID)
	if showErr != nil {
		addProblem(snapshot, "orca_dispatches", "orca_dispatch_failed", "could not resolve dispatch for task "+taskID)
		return
	}
	value := operationalhealthcontract.OrcaDispatch{RuntimeID: strings.TrimSpace(dispatch.RuntimeID), RunID: strings.TrimSpace(task.RunID), ID: strings.TrimSpace(dispatch.ID), TaskID: strings.TrimSpace(dispatch.TaskID), AssigneeHandle: strings.TrimSpace(dispatch.AssigneeHandle), Status: strings.TrimSpace(dispatch.Status)}
	if value.RuntimeID != snapshot.OrcaRuntimeID || value.ID == "" || value.TaskID != taskID || value.AssigneeHandle == "" || value.Status != "dispatched" {
		addProblem(snapshot, "orca_dispatches", "orca_dispatch_identity_mismatch", "dispatch identity does not match task "+taskID)
	}
	snapshot.Dispatches = append(snapshot.Dispatches, value)
	if value.ID != "" && value.TaskID == taskID {
		assignTaskDispatchID(snapshot.Tasks, taskKey, value.ID)
	}
}

func orcaDispatchedTaskAmbiguous(snapshot *corehealth.Snapshot, task port.OrcaTask, taskID string, count int) bool {
	return taskID == "" || strings.TrimSpace(task.Status) != "dispatched" || count != 1 || countTasksWithStatus(snapshot.Tasks, task.RunID, taskID, "dispatched") != 1
}

func assignTaskDispatchID(tasks []operationalhealthcontract.OrcaTask, taskKey, dispatchID string) {
	for index := range tasks {
		if operationalTaskKey(tasks[index].RunID, tasks[index].ID) == taskKey {
			tasks[index].DispatchID = dispatchID
		}
	}
}

func applyOrcaGates(snapshot *corehealth.Snapshot, readers *orcaRunReaders) {
	if readers.gatesErr != nil {
		addProblem(snapshot, "orca_gates", "orca_gates_failed", "Orca gate inventory failed")
		return
	}
	for _, value := range readers.gates {
		gate := operationalhealthcontract.OrcaGate{RuntimeID: strings.TrimSpace(value.RuntimeID), ID: strings.TrimSpace(value.ID), TaskID: strings.TrimSpace(value.TaskID), Status: strings.TrimSpace(value.Status)}
		if gate.RuntimeID != snapshot.OrcaRuntimeID {
			addProblem(snapshot, "orca_gates", "orca_gate_runtime_mismatch", "gate "+gate.ID+" runtime identity does not match")
		}
		snapshot.Gates = append(snapshot.Gates, gate)
	}
}

func (collector Collector) collectOrcaInbox(ctx context.Context, snapshot *corehealth.Snapshot) {
	inbox, err := collector.Orca.InboxPresence(ctx)
	if err != nil {
		addProblem(snapshot, "orca_inbox", "orca_inbox_failed", "Orca inbox presence inventory failed")
		return
	}
	snapshot.Messages = operationalhealthcontract.MessagePresence{RuntimeID: strings.TrimSpace(inbox.RuntimeID), Count: inbox.Count, Empty: inbox.RowCount == 0, CompleteAbsence: inbox.CompleteAbsence}
	if snapshot.Messages.RuntimeID != snapshot.OrcaRuntimeID || inbox.Count != inbox.RowCount || inbox.RowCount < 0 || inbox.RowCount > 1 {
		addProblem(snapshot, "orca_inbox", "orca_inbox_count_mismatch", "bounded Orca inbox count does not match returned rows")
	}
}
