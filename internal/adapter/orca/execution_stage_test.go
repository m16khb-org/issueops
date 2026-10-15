package orca

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"issueops/internal/port"
)

const (
	stageDispatchRequestID = "11111111-1111-4111-8111-111111111111"
	stagePromptRequestID   = "22222222-2222-4222-8222-222222222222"
)

// scriptedExecutionFake lets one Orca call fail while every other call keeps the
// shared executionFake behavior.
type scriptedExecutionFake struct {
	*executionFake
	listWorktreesErr, createWorktreeErr, createTerminalErr   error
	createRunErr, currentRunErr, useRunErr, createTaskErr    error
	dispatchErr, listTerminalsErr, listRunsErr, currentInvEr error
	tasksErr, dispatchShowErr                                error
}

func (f *scriptedExecutionFake) ListWorktrees(ctx context.Context, repo string) ([]port.OrcaWorktree, error) {
	if f.listWorktreesErr != nil {
		return nil, f.listWorktreesErr
	}
	return f.executionFake.ListWorktrees(ctx, repo)
}

func (f *scriptedExecutionFake) CreateWorktree(ctx context.Context, req port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error) {
	if f.createWorktreeErr != nil {
		return port.OrcaWorktree{}, f.createWorktreeErr
	}
	return f.executionFake.CreateWorktree(ctx, req)
}

func (f *scriptedExecutionFake) CreateTerminal(ctx context.Context, req port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error) {
	if f.createTerminalErr != nil {
		return port.OrcaTerminal{}, f.createTerminalErr
	}
	return f.executionFake.CreateTerminal(ctx, req)
}

func (f *scriptedExecutionFake) CreateRun(ctx context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error) {
	if f.createRunErr != nil {
		return port.OrcaRun{}, f.createRunErr
	}
	return f.executionFake.CreateRun(ctx, req)
}

func (f *scriptedExecutionFake) CurrentRun(ctx context.Context) (*port.OrcaRun, error) {
	if f.currentRunErr != nil {
		return nil, f.currentRunErr
	}
	return f.executionFake.CurrentRun(ctx)
}

func (f *scriptedExecutionFake) UseRun(ctx context.Context, runID string) (port.OrcaRun, error) {
	if f.useRunErr != nil {
		return port.OrcaRun{}, f.useRunErr
	}
	return f.executionFake.UseRun(ctx, runID)
}

func (f *scriptedExecutionFake) CreateTask(ctx context.Context, req port.OrcaCreateTaskRequest) (port.OrcaTask, error) {
	if f.createTaskErr != nil {
		return port.OrcaTask{}, f.createTaskErr
	}
	return f.executionFake.CreateTask(ctx, req)
}

func (f *scriptedExecutionFake) Dispatch(ctx context.Context, req port.OrcaDispatchRequest) (port.OrcaDispatch, error) {
	if f.dispatchErr != nil {
		return port.OrcaDispatch{}, f.dispatchErr
	}
	return f.executionFake.Dispatch(ctx, req)
}

func (f *scriptedExecutionFake) listTerminalsInventory(ctx context.Context, worktreeID string) (executionTerminalInventory, error) {
	if f.listTerminalsErr != nil {
		return executionTerminalInventory{}, f.listTerminalsErr
	}
	return f.executionFake.listTerminalsInventory(ctx, worktreeID)
}

func (f *scriptedExecutionFake) listRunsInventory(ctx context.Context) (executionRunInventory, error) {
	if f.listRunsErr != nil {
		return executionRunInventory{}, f.listRunsErr
	}
	return f.executionFake.listRunsInventory(ctx)
}

func (f *scriptedExecutionFake) currentRunInventory(ctx context.Context) (executionCurrentRunInventory, error) {
	if f.currentInvEr != nil {
		return executionCurrentRunInventory{}, f.currentInvEr
	}
	return f.executionFake.currentRunInventory(ctx)
}

func (f *scriptedExecutionFake) listRunTasksInventory(ctx context.Context, runID string, flags ...string) (executionTaskInventory, error) {
	if f.tasksErr != nil {
		return executionTaskInventory{}, f.tasksErr
	}
	return f.executionFake.listRunTasksInventory(ctx, runID, flags...)
}

func (f *scriptedExecutionFake) showDispatchInventory(ctx context.Context, taskID string) (executionDispatchInventory, error) {
	if f.dispatchShowErr != nil {
		return executionDispatchInventory{}, f.dispatchShowErr
	}
	return f.executionFake.showDispatchInventory(ctx, taskID)
}

// requestOnlyClient exposes only the mutation surface, so every inventory and
// delivery-identity type assertion fails.
type requestOnlyClient struct{ executionClient }

type stageFixture struct {
	workspace port.ExecutionWorkspaceRequest
	probe     port.ExecutionOrcaProbeRequest
	prepared  port.ExecutionOrcaWorkspaceReceipt
	launch    port.ExecutionOrcaLaunchRequest
}

func newStageFixture(t *testing.T) stageFixture {
	t.Helper()
	workspace, probe := executionFixture(t)
	launch := executionLaunchFixture(t, workspace.Root)
	return stageFixture{
		workspace: workspace, probe: probe, launch: launch,
		prepared: executionWorkspaceReceipt(workspace, executionWorktree(workspace, probe)),
	}
}

func (f stageFixture) omo() stageFixture {
	f.probe.Host = "omo"
	f.probe.Model = "chatgpt-subscription/gpt-6-sol"
	return f
}

// request builds the sealed request with exactly the receipts the stage requires.
func (f stageFixture) request(stage port.ExecutionOrcaIntentStage) port.ExecutionOrcaIntentRequest {
	request := port.ExecutionOrcaIntentRequest{Stage: stage, Marker: f.probe.Marker, Workspace: f.workspace, Probe: f.probe}
	if stage == port.ExecutionOrcaIntentWorktree {
		return request
	}
	prepared, launch := f.prepared, f.launch
	request.Prepared, request.Launch = &prepared, &launch
	switch stage {
	case port.ExecutionOrcaIntentRun:
		request.TerminalPTYID = "pty-69"
	case port.ExecutionOrcaIntentRunBind:
		request.TerminalPTYID, request.RunID = "pty-69", "run-69"
	case port.ExecutionOrcaIntentTask:
		request.TerminalPTYID, request.RunID, request.RunBound = "pty-69", "run-69", true
	case port.ExecutionOrcaIntentDispatch:
		request.TerminalPTYID, request.RunID, request.RunBound, request.TaskID = "pty-69", "run-69", true, "task-69"
	}
	return request
}

func (f stageFixture) fake() *executionFake {
	return &executionFake{
		workspace: f.workspace, probeRequest: f.probe,
		terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Connected: true, Writable: true}},
		dispatchResult: &port.OrcaDispatch{
			RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69",
			Status: "dispatched", Preamble: "official preamble", Injected: f.probe.Host != "omo", RequestID: stageDispatchRequestID,
		},
	}
}

func fastSettle(client executionClient) *ExecutionProvisioner {
	provisioner := NewExecutionClient(client)
	provisioner.terminalSettleBudget = time.Millisecond
	provisioner.terminalSettleInterval = time.Millisecond
	return provisioner
}

func TestInvokeIntentPropagatesOrcaFailuresWithoutRewriting(t *testing.T) {
	boom := &port.OrcaError{Code: "runtime_error", Detail: "boom"}
	fx := newStageFixture(t)
	tests := []struct {
		name   string
		stage  port.ExecutionOrcaIntentStage
		script func(*scriptedExecutionFake)
	}{
		{"worktree create", port.ExecutionOrcaIntentWorktree, func(f *scriptedExecutionFake) { f.createWorktreeErr = boom }},
		{"terminal create", port.ExecutionOrcaIntentTerminal, func(f *scriptedExecutionFake) { f.createTerminalErr = boom }},
		{"run create", port.ExecutionOrcaIntentRun, func(f *scriptedExecutionFake) { f.createRunErr = boom }},
		{"current run read", port.ExecutionOrcaIntentRunBind, func(f *scriptedExecutionFake) { f.currentRunErr = boom }},
		{"run use", port.ExecutionOrcaIntentRunBind, func(f *scriptedExecutionFake) { f.useRunErr = boom }},
		{"task create", port.ExecutionOrcaIntentTask, func(f *scriptedExecutionFake) { f.createTaskErr = boom }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &scriptedExecutionFake{executionFake: fx.fake()}
			tt.script(client)
			_, err := NewExecutionClient(client).InvokeIntent(context.Background(), fx.request(tt.stage))
			if !errors.Is(err, boom) {
				t.Fatalf("error = %v, want the Orca failure unchanged", err)
			}
		})
	}
}

func TestInvokeIntentRejectsCreatedResourcesThatDriftFromTheSeal(t *testing.T) {
	fx := newStageFixture(t)
	t.Run("worktree on another branch", func(t *testing.T) {
		drifted := executionWorktree(fx.workspace, fx.probe)
		drifted.Branch = "other"
		client := fx.fake()
		client.createdWorktree = &drifted
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), fx.request(port.ExecutionOrcaIntentWorktree))
		if orcaErr := requireOrcaError(t, err, "worktree_identity_mismatch"); !orcaErr.Invoked {
			t.Fatal("the worktree exists remotely and must report invoked")
		}
	})
	t.Run("run from another runtime", func(t *testing.T) {
		// The fake echoes the sealed objective, so shift the runtime instead.
		client := runtimeShiftedRuns{&scriptedExecutionFake{executionFake: fx.fake()}}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRun))
		if orcaErr := requireOrcaError(t, err, "run_identity_mismatch"); !orcaErr.Invoked {
			t.Fatal("the Run exists remotely and must report invoked")
		}
	})
	t.Run("task with another title", func(t *testing.T) {
		client := fx.fake()
		client.createdTask = &port.OrcaTask{RuntimeID: "runtime-69", RunID: "run-69", ID: "task-69", Title: "someone else's task", DisplayName: fx.workspace.Branch, Status: "ready"}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTask))
		if orcaErr := requireOrcaError(t, err, "task_identity_mismatch"); !orcaErr.Invoked {
			t.Fatal("the task exists remotely and must report invoked")
		}
	})
}

// runtimeShiftedRuns makes CreateRun answer from a different Orca runtime.
type runtimeShiftedRuns struct{ *scriptedExecutionFake }

func (r runtimeShiftedRuns) CreateRun(ctx context.Context, req port.OrcaCreateRunRequest) (port.OrcaRun, error) {
	run, err := r.scriptedExecutionFake.CreateRun(ctx, req)
	run.RuntimeID = "runtime-other"
	return run, err
}

func TestInvokeTerminalIntentReconcilesAgainstTheInventory(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentTerminal)

	t.Run("client without inventory access cannot reconcile", func(t *testing.T) {
		client := fx.fake()
		client.createdTerminal = &port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Title: "agent status", Connected: true, Writable: true}
		_, err := NewExecutionClient(requestOnlyClient{client}).InvokeIntent(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "terminal_identity_mismatch"); !orcaErr.Invoked || !strings.Contains(orcaErr.Detail, "does not match the sealed intent") {
			t.Fatalf("error = %#v", orcaErr)
		}
	})
	t.Run("created terminal with no identity cannot be reconciled", func(t *testing.T) {
		client := fx.fake()
		client.createdTerminal = &port.OrcaTerminal{RuntimeID: "runtime-69", WorktreeID: "wt-69", Connected: true, Writable: true}
		_, err := fastSettle(client).InvokeIntent(context.Background(), request)
		requireOrcaCode(t, err, "terminal_identity_mismatch")
		for _, call := range client.calls {
			if call == "list-terminals-inventory" {
				t.Fatalf("an identity-less terminal was searched for: %v", client.calls)
			}
		}
	})
	t.Run("inventory read failure stops reconciliation", func(t *testing.T) {
		client := &scriptedExecutionFake{executionFake: fx.fake(), listTerminalsErr: errors.New("inventory down")}
		client.createdTerminal = &port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69"}
		_, err := fastSettle(client).InvokeIntent(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "terminal_identity_mismatch"); !strings.Contains(orcaErr.Detail, "inventory down") {
			t.Fatalf("detail = %q", orcaErr.Detail)
		}
	})
	t.Run("a cancelled context interrupts the wait", func(t *testing.T) {
		client := fx.fake()
		client.createdTerminal = &port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69"}
		client.terminals = nil
		provisioner := NewExecutionClient(client)
		provisioner.terminalSettleBudget = 2 * time.Hour
		provisioner.terminalSettleInterval = time.Hour
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := provisioner.InvokeIntent(ctx, request)
		if orcaErr := requireOrcaError(t, err, "terminal_identity_mismatch"); !strings.Contains(orcaErr.Detail, context.Canceled.Error()) {
			t.Fatalf("detail = %q", orcaErr.Detail)
		}
	})
}

func countCalls(calls []string, name string) int {
	count := 0
	for _, call := range calls {
		if call == name {
			count++
		}
	}
	return count
}

func TestInvokeRunBindIntent(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentRunBind)
	bound := port.OrcaRun{RuntimeID: "runtime-69", ID: "run-69", Objective: fx.probe.Marker}

	t.Run("already bound run is not re-bound", func(t *testing.T) {
		client := fx.fake()
		client.currentRun = &bound
		receipt, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if err != nil || receipt.RunID != "run-69" || !receipt.RunBound {
			t.Fatalf("receipt=%#v err=%v", receipt, err)
		}
		if countCalls(client.calls, "use-run") != 0 {
			t.Fatalf("an already-current Run was re-bound: %v", client.calls)
		}
	})
	t.Run("another current run is replaced via use", func(t *testing.T) {
		client := fx.fake()
		client.currentRun = &port.OrcaRun{RuntimeID: "runtime-69", ID: "run-other", Objective: "other"}
		client.runs = []port.OrcaRun{bound}
		receipt, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if err != nil || receipt.RunID != "run-69" || !receipt.RunBound || countCalls(client.calls, "use-run") != 1 {
			t.Fatalf("receipt=%#v err=%v calls=%v", receipt, err, client.calls)
		}
	})
	t.Run("current run with another objective is a binding mismatch", func(t *testing.T) {
		client := fx.fake()
		client.currentRun = &port.OrcaRun{RuntimeID: "runtime-69", ID: "run-69", Objective: "different objective"}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "run_binding_mismatch"); !orcaErr.Invoked {
			t.Fatal("binding mismatch must report invoked")
		}
	})
}

func TestInvokeDispatchIntentFailureAttribution(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentDispatch)

	t.Run("dispatch failure is tagged with its call phase", func(t *testing.T) {
		client := &scriptedExecutionFake{executionFake: fx.fake(), dispatchErr: &port.OrcaError{Code: "runtime_error", Invoked: true}}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "runtime_error"); orcaErr.CallPhase != "orca_dispatch" {
			t.Fatalf("call phase = %q", orcaErr.CallPhase)
		}
	})
	t.Run("untyped dispatch failure passes through", func(t *testing.T) {
		boom := errors.New("socket closed")
		client := &scriptedExecutionFake{executionFake: fx.fake(), dispatchErr: boom}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if !errors.Is(err, boom) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("dispatch to another terminal is an identity mismatch", func(t *testing.T) {
		client := fx.fake()
		client.dispatchResult.AssigneeHandle = "term-other"
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "dispatch_identity_mismatch"); !orcaErr.Invoked {
			t.Fatal("the dispatch exists remotely and must report invoked")
		}
	})
	t.Run("injected mismatch for a claude owner", func(t *testing.T) {
		client := fx.fake()
		client.dispatchResult.Injected = false
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		requireOrcaCode(t, err, "dispatch_identity_mismatch")
	})
	t.Run("a non-injected dispatch must return the preamble", func(t *testing.T) {
		omo := fx.omo()
		client := omo.fake()
		client.dispatchResult.Preamble = " "
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), omo.request(port.ExecutionOrcaIntentDispatch))
		requireOrcaCode(t, err, "dispatch_identity_mismatch")
		if countCalls(client.calls, "send-terminal-prompt") != 0 {
			t.Fatal("a prompt was sent without a preamble")
		}
	})
	t.Run("a durable request identity is mandatory", func(t *testing.T) {
		client := fx.fake()
		client.dispatchResult.RequestID = ""
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		if orcaErr := requireOrcaError(t, err, "dispatch_request_identity_mismatch"); orcaErr.CallPhase != "orca_dispatch" {
			t.Fatalf("call phase = %q", orcaErr.CallPhase)
		}
	})
}

func TestInvokeDispatchIntentObserverFailures(t *testing.T) {
	failOn := func(kind string, phase port.ExecutionOrcaCallPhase) func(port.ExecutionOrcaCallObservation) error {
		return func(observation port.ExecutionOrcaCallObservation) error {
			if observation.CallKind == kind && observation.Phase == phase {
				return fmt.Errorf("journal full during %s %s", kind, phase)
			}
			return nil
		}
	}
	omo := newStageFixture(t).omo()
	tests := []struct {
		name         string
		kind         string
		phase        port.ExecutionOrcaCallPhase
		wantInvoked  bool
		wantPhase    string
		wantCalls    []string
		wantDispatch string
	}{
		{"before dispatch", "dispatch", port.ExecutionOrcaCallStaged, false, "orca_dispatch", []string{"list-terminals-inventory"}, ""},
		{"after dispatch", "dispatch", port.ExecutionOrcaCallCompleted, true, "orca_dispatch", []string{"list-terminals-inventory", "dispatch"}, stageDispatchRequestID},
		{"before prompt", "prompt", port.ExecutionOrcaCallStaged, true, "terminal_send", []string{"list-terminals-inventory", "dispatch"}, stageDispatchRequestID},
		{"after prompt", "prompt", port.ExecutionOrcaCallCompleted, true, "terminal_send", []string{"list-terminals-inventory", "dispatch", "send-terminal-prompt"}, stageDispatchRequestID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := omo.fake()
			_, err := NewExecutionClient(client).InvokeIntentObserved(context.Background(), omo.request(port.ExecutionOrcaIntentDispatch), failOn(tt.kind, tt.phase))
			orcaErr := requireOrcaError(t, err, "delivery_observation_failed")
			if orcaErr.Invoked != tt.wantInvoked || orcaErr.CallPhase != tt.wantPhase || orcaErr.DispatchRequestID != tt.wantDispatch ||
				!strings.Contains(orcaErr.Detail, "journal full") {
				t.Fatalf("observer failure = %#v", orcaErr)
			}
			if strings.Join(client.calls, ",") != strings.Join(tt.wantCalls, ",") {
				t.Fatalf("calls = %v, want %v", client.calls, tt.wantCalls)
			}
		})
	}
}

func TestInvokeDispatchPromptDeliveryFailures(t *testing.T) {
	omo := newStageFixture(t).omo()
	t.Run("typed send failure is attributed to the terminal send", func(t *testing.T) {
		client := omo.fake()
		client.promptErr = &port.OrcaError{Code: "terminal_not_writable"}
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), omo.request(port.ExecutionOrcaIntentDispatch))
		orcaErr := requireOrcaError(t, err, "terminal_not_writable")
		if !orcaErr.Invoked || orcaErr.CallPhase != "terminal_send" || orcaErr.DispatchRequestID != stageDispatchRequestID {
			t.Fatalf("send failure = %#v", orcaErr)
		}
	})
	t.Run("untyped send failure passes through", func(t *testing.T) {
		boom := errors.New("pipe closed")
		client := omo.fake()
		client.promptErr = boom
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), omo.request(port.ExecutionOrcaIntentDispatch))
		if !errors.Is(err, boom) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("receipt id differs from the pinned retry id", func(t *testing.T) {
		client := omo.fake()
		client.promptReceipt = &port.OrcaPromptReceipt{RequestID: "33333333-3333-4333-8333-333333333333", Stages: []string{"input_accepted"}, Provider: "omo", ProcessIncarnation: "p", Generation: 1, BaselineWorkingSequence: new(uint64(0))}
		request := omo.request(port.ExecutionOrcaIntentDispatch)
		request.PromptRetryRequestID = stagePromptRequestID
		_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
		orcaErr := requireOrcaError(t, err, "terminal_prompt_request_identity_mismatch")
		if orcaErr.OrchestrationRequestID != "33333333-3333-4333-8333-333333333333" || orcaErr.DispatchRequestID != stageDispatchRequestID {
			t.Fatalf("identities = %#v", orcaErr)
		}
	})
	t.Run("matching receipt is attached to the dispatch receipt", func(t *testing.T) {
		client := omo.fake()
		receipt, err := NewExecutionClient(client).InvokeIntent(context.Background(), omo.request(port.ExecutionOrcaIntentDispatch))
		if err != nil || receipt.PromptReceipt == nil || receipt.PromptReceipt.ProcessIncarnation != "incarnation-1" || receipt.RequestID != stageDispatchRequestID {
			t.Fatalf("receipt=%#v err=%v", receipt, err)
		}
	})
	t.Run("claude owners never receive a prompt", func(t *testing.T) {
		fx := newStageFixture(t)
		client := fx.fake()
		receipt, err := NewExecutionClient(client).InvokeIntent(context.Background(), fx.request(port.ExecutionOrcaIntentDispatch))
		if err != nil || receipt.PromptReceipt != nil || countCalls(client.calls, "send-terminal-prompt") != 0 {
			t.Fatalf("receipt=%#v err=%v calls=%v", receipt, err, client.calls)
		}
		if !client.dispatchRequest.Inject {
			t.Fatal("non-Omo owners must receive the preamble through dispatch injection")
		}
	})
}

func TestObserveDeliveryWithoutObserverIsANoop(t *testing.T) {
	if err := observeDelivery(nil, "dispatch", port.ExecutionOrcaCallStaged, port.ExecutionOrcaIntentReceipt{}, port.OrcaError{}); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionPreflightErrorKeepsNilAndMarksNotInvoked(t *testing.T) {
	if executionPreflightError(nil) != nil {
		t.Fatal("a nil error must stay nil")
	}
	orcaErr := requireOrcaError(t, executionPreflightError(errors.New("bad seal")), "intent_preflight_rejected")
	if orcaErr.Invoked || orcaErr.Detail != "bad seal" {
		t.Fatalf("preflight = %#v", orcaErr)
	}
}
