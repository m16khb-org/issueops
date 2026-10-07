package orca

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sync/errgroup"
	"os"
	"slices"
	"strings"

	deliverycontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func (c *Client) ListRuns(ctx context.Context) ([]port.OrcaRun, error) {
	inventory, err := c.listRunsInventory(ctx)
	return inventory.Rows, err
}

func (c *Client) ListRunInventory(ctx context.Context) (port.OrcaRunInventory, error) {
	status, err := c.Status(ctx)
	if err != nil {
		return port.OrcaRunInventory{}, err
	}
	runs, err := c.listRunsInventory(ctx)
	if err != nil {
		return port.OrcaRunInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(runs.RuntimeID, status.RuntimeID); err != nil {
		return port.OrcaRunInventory{}, err
	}
	return port.OrcaRunInventory{RuntimeID: runs.RuntimeID, Runs: runs.Rows}, nil
}

func (c *Client) ListAllTasksFromRuns(ctx context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return c.listTasksFromRuns(ctx, inventory, "--brief")
}

func (c *Client) ListDispatchedTasksFromRuns(ctx context.Context, inventory port.OrcaRunInventory) ([]port.OrcaTask, error) {
	return c.listTasksFromRuns(ctx, inventory, "--status", "dispatched")
}

func (c *Client) ListGatesFromRuns(ctx context.Context, inventory port.OrcaRunInventory) ([]port.OrcaGate, error) {
	runs, err := validateRunInventory(inventory)
	if err != nil {
		return nil, err
	}
	entries, errs := readRunsBounded(ctx, runs, func(ctx context.Context, run port.OrcaRun) (executionGateInventory, error) {
		return c.listRunGatesInventory(ctx, run.ID)
	})
	result := make([]port.OrcaGate, 0)
	seen := make(map[string]struct{})
	for index, entry := range entries {
		if errs[index] != nil {
			return nil, errs[index]
		}
		if err := validateExecutionInventoryRuntime(entry.RuntimeID, inventory.RuntimeID); err != nil {
			return nil, err
		}
		for _, gate := range entry.Rows {
			if _, duplicate := seen[gate.ID]; duplicate {
				return nil, &port.OrcaError{Code: "gate_inventory_ambiguous", Detail: "Orca returned a duplicate gate identity", Invoked: true}
			}
			seen[gate.ID] = struct{}{}
			result = append(result, gate)
		}
	}
	return result, nil
}

func (c *Client) listTasksFromRuns(ctx context.Context, inventory port.OrcaRunInventory, flags ...string) ([]port.OrcaTask, error) {
	runs, err := validateRunInventory(inventory)
	if err != nil {
		return nil, err
	}
	entries, errs := readRunsBounded(ctx, runs, func(ctx context.Context, run port.OrcaRun) (executionTaskInventory, error) {
		return c.listRunTasksInventory(ctx, run.ID, flags...)
	})
	result := make([]port.OrcaTask, 0)
	seen := make(map[string]struct{})
	for index, entry := range entries {
		if errs[index] != nil {
			return nil, errs[index]
		}
		if err := validateExecutionInventoryRuntime(entry.RuntimeID, inventory.RuntimeID); err != nil {
			return nil, err
		}
		for _, task := range entry.Rows {
			key := task.RunID + "\x00" + task.ID
			if _, duplicate := seen[key]; duplicate {
				return nil, &port.OrcaError{Code: "task_inventory_ambiguous", Detail: "Orca returned a duplicate task identity in one Run", Invoked: true}
			}
			seen[key] = struct{}{}
			result = append(result, task)
		}
	}
	return result, nil
}

func validateRunInventory(inventory port.OrcaRunInventory) ([]port.OrcaRun, error) {
	if strings.TrimSpace(inventory.RuntimeID) == "" || inventory.RuntimeID != strings.TrimSpace(inventory.RuntimeID) {
		return nil, &port.OrcaError{Code: "run_inventory_runtime_invalid", Detail: "Orca Run inventory requires a canonical runtime identity"}
	}
	runs := append([]port.OrcaRun(nil), inventory.Runs...)
	seen := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		if _, err := validateRunID(run.ID); err != nil || run.RuntimeID != inventory.RuntimeID || strings.TrimSpace(run.Objective) == "" || run.Objective != strings.TrimSpace(run.Objective) {
			return nil, &port.OrcaError{Code: "run_inventory_identity_invalid", Detail: "Orca Run inventory contains an invalid Run identity"}
		}
		if _, duplicate := seen[run.ID]; duplicate {
			return nil, &port.OrcaError{Code: "run_inventory_ambiguous", Detail: "Orca Run inventory contains a duplicate Run identity"}
		}
		seen[run.ID] = struct{}{}
	}
	return runs, nil
}

func readRunsBounded[T any](ctx context.Context, runs []port.OrcaRun, read func(context.Context, port.OrcaRun) (T, error)) ([]T, []error) {
	values := make([]T, len(runs))
	errs := make([]error, len(runs))
	workers := min(8, len(runs))
	if workers == 0 {
		return values, errs
	}
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(workers)
	for index := range runs {
		index := index
		group.Go(func() error {
			values[index], errs[index] = read(groupCtx, runs[index])
			return nil
		})
	}
	_ = group.Wait()
	return values, errs
}

func (c *Client) listRunsInventory(ctx context.Context) (executionRunInventory, error) {
	argv := []string{"orca", "orchestration", "run-list", "--json"}
	result := make([]port.OrcaRun, 0)
	seenRuns := make(map[string]struct{})
	seenCursors := make(map[string]struct{})
	runtimeID := ""
	for {
		var payload struct {
			Runs       *[]runPayload   `json:"runs"`
			NextCursor json.RawMessage `json:"nextCursor"`
		}
		pageRuntimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
		if err != nil {
			return executionRunInventory{}, err
		}
		if strings.TrimSpace(pageRuntimeID) == "" || pageRuntimeID != strings.TrimSpace(pageRuntimeID) {
			return executionRunInventory{}, &port.OrcaError{Code: "run_inventory_runtime_invalid", Detail: "Orca Run list page has no canonical runtime identity", Invoked: true}
		}
		if runtimeID == "" {
			runtimeID = pageRuntimeID
		} else if err := validateExecutionInventoryRuntime(pageRuntimeID, runtimeID); err != nil {
			return executionRunInventory{}, err
		}
		if payload.Runs == nil {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list completeness metadata is missing", Invoked: true}
		}
		for _, row := range *payload.Runs {
			value, err := row.portValue(runtimeID)
			if err != nil {
				return executionRunInventory{}, err
			}
			if _, duplicate := seenRuns[value.ID]; duplicate {
				return executionRunInventory{}, &port.OrcaError{Code: "run_inventory_ambiguous", Detail: "Orca returned a duplicate Run identity", Invoked: true}
			}
			seenRuns[value.ID] = struct{}{}
			result = append(result, value)
		}
		nextCursor := strings.TrimSpace(string(payload.NextCursor))
		if nextCursor == "" {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list nextCursor completeness metadata is missing", Invoked: true}
		}
		if nextCursor == "null" {
			break
		}
		var cursor string
		if json.Unmarshal(payload.NextCursor, &cursor) != nil || strings.TrimSpace(cursor) == "" || cursor != strings.TrimSpace(cursor) {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list nextCursor is invalid", Invoked: true}
		}
		if _, duplicate := seenCursors[cursor]; duplicate {
			return executionRunInventory{}, &port.OrcaError{Code: "incomplete_list", Detail: "Orca Run list nextCursor repeated", Invoked: true}
		}
		seenCursors[cursor] = struct{}{}
		argv = []string{"orca", "orchestration", "run-list", "--cursor", cursor, "--json"}
	}
	slices.SortFunc(result, func(left, right port.OrcaRun) int {
		return strings.Compare(left.ID, right.ID)
	})
	return executionRunInventory{RuntimeID: runtimeID, Rows: result}, nil
}

func (c *Client) CreateRun(ctx context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error) {
	objective := strings.TrimSpace(req.Objective)
	if objective == "" || objective != req.Objective || len(objective) > 4096 || strings.ContainsRune(objective, 0) {
		return port.OrcaRun{}, &port.OrcaError{Code: "run_objective_invalid"}
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaRun{}, err
	}
	return c.runMutation(ctx, []string{"orca", "orchestration", "run-create", "--objective", objective, "--json"})
}

func (c *Client) CurrentRun(ctx context.Context) (*port.OrcaRun, error) {
	inventory, err := c.currentRunInventory(ctx)
	return inventory.Run, err
}

func (c *Client) currentRunInventory(ctx context.Context) (executionCurrentRunInventory, error) {
	if _, err := currentCoordinatorHandle(); err != nil {
		return executionCurrentRunInventory{}, err
	}
	var payload struct {
		Run json.RawMessage `json:"run"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "run-current", "--json"}, &payload)
	if err != nil {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, err
	}
	projection := strings.TrimSpace(string(payload.Run))
	if projection == "" {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, &port.OrcaError{Code: "incomplete_run_current", Detail: "Orca current Run projection is missing", Invoked: true}
	}
	if projection == "null" {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, nil
	}
	var row runPayload
	if err := json.Unmarshal(payload.Run, &row); err != nil {
		return executionCurrentRunInventory{RuntimeID: runtimeID}, &port.OrcaError{Code: "run_identity_invalid", Detail: "Orca current Run projection is malformed", Invoked: true}
	}
	value, err := row.portValue(runtimeID)
	if err != nil {
		return executionCurrentRunInventory{}, err
	}
	return executionCurrentRunInventory{RuntimeID: runtimeID, Run: &value}, nil
}

func (c *Client) UseRun(ctx context.Context, runID string) (port.OrcaRun, error) {
	runID, err := validateRunID(runID)
	if err != nil {
		return port.OrcaRun{}, err
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaRun{}, err
	}
	used, err := c.runMutation(ctx, []string{"orca", "orchestration", "run-use", "--id", runID, "--json"})
	if err == nil && used.ID != runID {
		return port.OrcaRun{}, &port.OrcaError{Code: "run_binding_mismatch", Detail: "Orca bound a different Run", Invoked: true}
	}
	return used, err
}

func (c *Client) runMutation(ctx context.Context, argv []string) (port.OrcaRun, error) {
	var payload struct {
		Run runPayload `json:"run"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return port.OrcaRun{}, err
	}
	return payload.Run.portValue(runtimeID)
}

func currentCoordinatorHandle() (string, error) {
	raw := os.Getenv("ORCA_TERMINAL_HANDLE")
	handle := strings.TrimSpace(raw)
	if raw != handle || !concreteTerminalHandlePattern.MatchString(handle) || len(handle) > 256 {
		return "", &port.OrcaError{Code: "coordinator_identity_unavailable", Detail: "ORCA_TERMINAL_HANDLE must identify the current concrete coordinator terminal"}
	}
	return handle, nil
}

func validateRunID(runID string) (string, error) {
	raw := runID
	runID = strings.TrimSpace(raw)
	if raw != runID || runID == "" || len(runID) > 1024 || strings.ContainsRune(runID, 0) {
		return "", &port.OrcaError{Code: "run_identity_invalid"}
	}
	return runID, nil
}

func (c *Client) listRunTasksInventory(ctx context.Context, runID string, flags ...string) (executionTaskInventory, error) {
	runID, err := validateRunID(runID)
	if err != nil {
		return executionTaskInventory{}, err
	}
	argv := append([]string{"orca", "orchestration", "task-list"}, flags...)
	argv = append(argv, "--run", runID, "--json")
	var payload struct {
		Tasks []taskPayload `json:"tasks"`
		Count *int          `json:"count"`
		RunID *string       `json:"runId"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, argv, &payload)
	if err != nil {
		return executionTaskInventory{}, err
	}
	if err := requireReturnedRunID("task", runID, payload.RunID); err != nil {
		return executionTaskInventory{}, err
	}
	if err := requireReturnedCount("task", len(payload.Tasks), payload.Count); err != nil {
		return executionTaskInventory{}, err
	}
	result := make([]port.OrcaTask, 0, len(payload.Tasks))
	for _, task := range payload.Tasks {
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Status) == "" {
			return executionTaskInventory{}, fmt.Errorf("Orca task row identity is incomplete")
		}
		value := task.portValue()
		value.RuntimeID = runtimeID
		if value.RunID != "" && value.RunID != runID {
			return executionTaskInventory{}, &port.OrcaError{Code: "task_run_mismatch", Detail: "Orca returned a task from a different Run", Invoked: true}
		}
		value.RunID = runID
		result = append(result, value)
	}
	return executionTaskInventory{RuntimeID: runtimeID, Rows: result}, nil
}

func (c *Client) listRunGatesInventory(ctx context.Context, runID string) (executionGateInventory, error) {
	runID, err := validateRunID(runID)
	if err != nil {
		return executionGateInventory{}, err
	}
	var payload struct {
		Gates []struct {
			ID     string `json:"id"`
			TaskID string `json:"task_id"`
			Status string `json:"status"`
		} `json:"gates"`
		Count *int    `json:"count"`
		RunID *string `json:"runId"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "gate-list", "--run", runID, "--json"}, &payload)
	if err != nil {
		return executionGateInventory{}, err
	}
	if err := requireReturnedRunID("gate", runID, payload.RunID); err != nil {
		return executionGateInventory{}, err
	}
	if err := requireReturnedCount("gate", len(payload.Gates), payload.Count); err != nil {
		return executionGateInventory{}, err
	}
	result := make([]port.OrcaGate, 0, len(payload.Gates))
	for _, gate := range payload.Gates {
		if strings.TrimSpace(gate.ID) == "" || strings.TrimSpace(gate.TaskID) == "" || strings.TrimSpace(gate.Status) == "" {
			return executionGateInventory{}, fmt.Errorf("Orca gate row identity is incomplete")
		}
		result = append(result, port.OrcaGate{RuntimeID: runtimeID, ID: gate.ID, TaskID: gate.TaskID, Status: gate.Status})
	}
	return executionGateInventory{RuntimeID: runtimeID, Rows: result}, nil
}

func (c *Client) InboxPresence(ctx context.Context) (port.OrcaInboxPresence, error) {
	var payload struct {
		Messages []struct{} `json:"messages"`
		Count    *int       `json:"count"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "inbox", "--limit", "1", "--json"}, &payload)
	if err != nil {
		return port.OrcaInboxPresence{}, err
	}
	if payload.Count == nil {
		return port.OrcaInboxPresence{}, fmt.Errorf("Orca inbox completeness metadata is missing")
	}
	count := *payload.Count
	rows := len(payload.Messages)
	return port.OrcaInboxPresence{RuntimeID: runtimeID, Count: count, RowCount: rows, CompleteAbsence: count == 0 && rows == 0}, nil
}

func (c *Client) CreateTask(ctx context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	runID, err := validateRunID(req.RunID)
	if err != nil {
		return port.OrcaTask{}, err
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaTask{}, err
	}
	argv := []string{"orca", "orchestration", "task-create", "--spec", req.Spec, "--task-title", req.Title, "--display-name", req.DisplayName, "--run", runID, "--json"}
	var payload struct {
		Task taskPayload `json:"task"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	created := payload.Task.portValue()
	created.RuntimeID = runtimeID
	if err == nil && created.RunID != "" && created.RunID != runID {
		return port.OrcaTask{}, &port.OrcaError{Code: "task_run_mismatch", Invoked: true}
	}
	created.RunID = runID
	return created, err
}

func (c *Client) Dispatch(ctx context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	runID, err := validateRunID(req.RunID)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if err := deliverycontract.ValidateOrcaRetryRequestID(req.RetryRequestID); err != nil {
		return port.OrcaDispatch{}, &port.OrcaError{Code: "request_identity_invalid", Detail: err.Error()}
	}
	if _, err := currentCoordinatorHandle(); err != nil {
		return port.OrcaDispatch{}, err
	}
	argv := []string{"orca", "orchestration", "dispatch", "--task", req.TaskID, "--to", req.ToHandle, "--run", runID}
	if req.Inject {
		argv = append(argv, "--inject")
	}
	if req.ReturnPreamble {
		argv = append(argv, "--return-preamble")
	}
	if requestID := strings.TrimSpace(req.RetryRequestID); requestID != "" {
		argv = append(argv, "--retry-request", requestID)
	}
	argv = append(argv, "--json")
	dispatch, err := c.dispatchResult(ctx, argv)
	if err != nil {
		return dispatch, err
	}
	if err := port.ValidateOrcaDurableRequestID(dispatch.RequestID, req.RetryRequestID); err != nil {
		return port.OrcaDispatch{}, &port.OrcaError{
			Code: "dispatch_request_identity_mismatch", Detail: err.Error(), Invoked: true,
			OrchestrationRequestID: strings.TrimSpace(dispatch.RequestID),
		}
	}
	return dispatch, nil
}

func (c *Client) ShowDispatch(ctx context.Context, taskID string) (port.OrcaDispatch, error) {
	return c.dispatchResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--json"})
}

func (c *Client) showDispatchInventory(ctx context.Context, taskID string) (executionDispatchInventory, error) {
	return c.dispatchInventoryResult(ctx, []string{"orca", "orchestration", "dispatch-show", "--task", taskID, "--json"})
}

func (c *Client) ShowRequest(ctx context.Context, requestID string) (port.OrcaRequestObservation, error) {
	requestID = strings.TrimSpace(requestID)
	if err := deliverycontract.ValidateOrcaRequestID(requestID); err != nil {
		return port.OrcaRequestObservation{}, &port.OrcaError{Code: "request_identity_invalid"}
	}
	var payload struct {
		RequestID string `json:"requestId"`
		State     string `json:"state"`
		Method    string `json:"method"`
	}
	runtimeID, err := c.runJSON(ctx, "", readTimeout, []string{"orca", "orchestration", "request-show", "--request", requestID, "--json"}, &payload)
	if err != nil {
		return port.OrcaRequestObservation{}, err
	}
	return port.OrcaRequestObservation{RuntimeID: runtimeID, RequestID: payload.RequestID, Status: payload.State, Method: payload.Method}, nil
}

func (c *Client) dispatchResult(ctx context.Context, argv []string) (port.OrcaDispatch, error) {
	inventory, err := c.dispatchInventoryResult(ctx, argv)
	if err != nil {
		return port.OrcaDispatch{}, err
	}
	if inventory.Dispatch == nil {
		return port.OrcaDispatch{}, &port.OrcaError{Code: "not_found"}
	}
	return *inventory.Dispatch, nil
}

func (c *Client) dispatchInventoryResult(ctx context.Context, argv []string) (executionDispatchInventory, error) {
	var payload struct {
		Dispatch *struct {
			ID             string `json:"id"`
			TaskID         string `json:"task_id"`
			AssigneeHandle string `json:"assignee_handle"`
			Status         string `json:"status"`
		} `json:"dispatch"`
		Injected bool   `json:"injected"`
		Preamble string `json:"preamble"`
		Mutation struct {
			RequestID string `json:"requestId"`
		} `json:"mutation"`
	}
	runtimeID, err := c.runJSON(ctx, "", createTimeout, argv, &payload)
	if err != nil {
		return executionDispatchInventory{}, err
	}
	if payload.Dispatch == nil {
		return executionDispatchInventory{RuntimeID: runtimeID}, nil
	}
	dispatch := port.OrcaDispatch{RuntimeID: runtimeID, ID: payload.Dispatch.ID, TaskID: payload.Dispatch.TaskID, AssigneeHandle: payload.Dispatch.AssigneeHandle, Status: payload.Dispatch.Status, Injected: payload.Injected, Preamble: payload.Preamble, RequestID: strings.TrimSpace(payload.Mutation.RequestID)}
	return executionDispatchInventory{RuntimeID: runtimeID, Dispatch: &dispatch}, nil
}
