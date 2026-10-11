package orca

import (
	"context"
	"errors"
	"strings"
	"testing"

	"issueops/internal/port"
)

// probeClient records the probe request and answers with a canned result.
type probeClient struct {
	*executionFake
	got    port.OrcaProbeRequest
	result port.OrcaProbeResult
	err    error
}

func (c *probeClient) Probe(_ context.Context, req port.OrcaProbeRequest) (port.OrcaProbeResult, error) {
	c.got = req
	return c.result, c.err
}

// identityClient adds the delivery-identity surface to the shared fake.
type identityClient struct {
	*executionFake
	identity    port.ExecutionOrcaDeliveryIdentity
	identityErr error
	observation port.OrcaRequestObservation
	observeErr  error
	observed    string
}

func (c *identityClient) DeliveryIdentity(context.Context) (port.ExecutionOrcaDeliveryIdentity, error) {
	return c.identity, c.identityErr
}

func (c *identityClient) ShowRequest(_ context.Context, requestID string) (port.OrcaRequestObservation, error) {
	c.observed = requestID
	return c.observation, c.observeErr
}

// inventoryOnlyClient has the inventory reads but no terminal-detail read.
type inventoryOnlyClient struct {
	executionClient
	inventory executionInventoryClient
}

func (c inventoryOnlyClient) listTerminalsInventory(ctx context.Context, id string) (executionTerminalInventory, error) {
	return c.inventory.listTerminalsInventory(ctx, id)
}

func (c inventoryOnlyClient) listRunTasksInventory(ctx context.Context, run string, flags ...string) (executionTaskInventory, error) {
	return c.inventory.listRunTasksInventory(ctx, run, flags...)
}

func (c inventoryOnlyClient) showDispatchInventory(ctx context.Context, task string) (executionDispatchInventory, error) {
	return c.inventory.showDispatchInventory(ctx, task)
}

func TestExecutionProbeMapsRequestAndResult(t *testing.T) {
	var unavailable *ExecutionProvisioner
	got, err := unavailable.Probe(context.Background(), port.ExecutionOrcaProbeRequest{})
	if err != nil || got.Code != "orca_adapter_unavailable" || got.Available || got.Ready {
		t.Fatalf("nil provisioner probe = %#v, %v", got, err)
	}
	got, err = (&ExecutionProvisioner{}).Probe(context.Background(), port.ExecutionOrcaProbeRequest{})
	if err != nil || got.Code != "orca_adapter_unavailable" {
		t.Fatalf("client-less probe = %#v, %v", got, err)
	}

	client := &probeClient{executionFake: &executionFake{}, result: port.OrcaProbeResult{Available: true, Ready: true, Code: "ok-code", RuntimeID: "dropped"}}
	got, err = NewExecutionClient(client).Probe(context.Background(), port.ExecutionOrcaProbeRequest{Repo: "/repo", Host: "omo", Provider: "gitlab", Model: "m"})
	if err != nil || got != (port.ExecutionOrcaProbeResult{Available: true, Ready: true, Code: "ok-code"}) {
		t.Fatalf("probe result = %#v, %v", got, err)
	}
	if client.got != (port.OrcaProbeRequest{Repo: "/repo", Agent: "omo", Provider: "gitlab"}) {
		t.Fatalf("probe request = %#v", client.got)
	}

	boom := errors.New("probe exploded")
	client = &probeClient{executionFake: &executionFake{}, result: port.OrcaProbeResult{Code: "status_failed"}, err: boom}
	got, err = NewExecutionClient(client).Probe(context.Background(), port.ExecutionOrcaProbeRequest{})
	if !errors.Is(err, boom) || got.Code != "status_failed" {
		t.Fatalf("probe failure = %#v, %v", got, err)
	}
}

func TestInspectDeliveryIdentity(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentDispatch)
	identity := port.ExecutionOrcaDeliveryIdentity{LauncherPath: "/opt/orca", Version: "1", RuntimeID: "runtime-69", MachineID: "host", TargetIdentity: "local"}
	newClient := func() *identityClient { return &identityClient{executionFake: fx.fake(), identity: identity} }

	t.Run("client without identity observation", func(t *testing.T) {
		_, err := NewExecutionClient(fx.fake()).InspectDeliveryIdentity(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "delivery identity observation is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("identity failure", func(t *testing.T) {
		client := newClient()
		client.identityErr = &port.OrcaError{Code: "orca_identity_unresolved"}
		_, err := NewExecutionClient(client).InspectDeliveryIdentity(context.Background(), request)
		requireOrcaCode(t, err, "orca_identity_unresolved")
	})
	t.Run("runtime changed since the seal", func(t *testing.T) {
		client := newClient()
		client.identity.RuntimeID = "runtime-other"
		_, err := NewExecutionClient(client).InspectDeliveryIdentity(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("no sealed worktree", func(t *testing.T) {
		unsealed := request
		unsealed.Prepared = nil
		_, err := NewExecutionClient(newClient()).InspectDeliveryIdentity(context.Background(), unsealed)
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("owner terminal is gone", func(t *testing.T) {
		client := newClient()
		client.terminals = nil
		_, err := NewExecutionClient(client).InspectDeliveryIdentity(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "owner terminal is absent") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("identity gains the live terminal", func(t *testing.T) {
		got, err := NewExecutionClient(newClient()).InspectDeliveryIdentity(context.Background(), request)
		want := identity
		want.TerminalPTYID, want.TerminalHandle = "pty-69", "term-69"
		if err != nil || got != want {
			t.Fatalf("identity = %+v, %v; want %+v", got, err, want)
		}
	})
}

func TestObserveRequest(t *testing.T) {
	t.Run("malformed id never reaches the client", func(t *testing.T) {
		client := &identityClient{executionFake: &executionFake{}}
		_, err := NewExecutionClient(client).ObserveRequest(context.Background(), "nope")
		if err == nil || client.observed != "" {
			t.Fatalf("err=%v observed=%q", err, client.observed)
		}
	})
	t.Run("client without request observation", func(t *testing.T) {
		_, err := NewExecutionClient(&executionFake{}).ObserveRequest(context.Background(), stageDispatchRequestID)
		if err == nil || !strings.Contains(err.Error(), "request observation is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("observation is returned as is", func(t *testing.T) {
		want := port.OrcaRequestObservation{RuntimeID: "rt", RequestID: stageDispatchRequestID, Status: "completed", Method: "dispatch"}
		client := &identityClient{executionFake: &executionFake{}, observation: want}
		got, err := NewExecutionClient(client).ObserveRequest(context.Background(), stageDispatchRequestID)
		if err != nil || got != want || client.observed != stageDispatchRequestID {
			t.Fatalf("got=%#v err=%v observed=%q", got, err, client.observed)
		}
	})
	t.Run("client failure is returned", func(t *testing.T) {
		client := &identityClient{executionFake: &executionFake{}, observeErr: &port.OrcaError{Code: "not_found"}}
		_, err := NewExecutionClient(client).ObserveRequest(context.Background(), stageDispatchRequestID)
		requireOrcaCode(t, err, "not_found")
	})
}

func TestInspectDeliveryDispatchBranches(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentDispatch)
	request.RetryRequestID = stageDispatchRequestID
	dispatch := port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69", Status: "dispatched", RequestID: stageDispatchRequestID}

	t.Run("durable request id is required", func(t *testing.T) {
		for _, id := range []string{"", "not-a-uuid"} {
			unpinned := request
			unpinned.RetryRequestID = id
			client := fx.fake()
			_, _, err := NewExecutionClient(client).InspectDeliveryDispatch(context.Background(), unpinned)
			if err == nil || !strings.Contains(err.Error(), "requires a durable request ID") || len(client.calls) != 0 {
				t.Fatalf("id %q: error=%v calls=%v", id, err, client.calls)
			}
		}
	})
	t.Run("owner terminal cannot be resolved", func(t *testing.T) {
		client := fx.fake()
		client.terminals = nil
		client.dispatch = dispatch
		_, exists, err := NewExecutionClient(client).InspectDeliveryDispatch(context.Background(), request)
		if err == nil || exists {
			t.Fatalf("exists=%t err=%v", exists, err)
		}
	})
	t.Run("dispatch read failure", func(t *testing.T) {
		client := &scriptedExecutionFake{executionFake: fx.fake(), dispatchShowErr: &port.OrcaError{Code: "runtime_error"}}
		_, _, err := NewExecutionClient(client).InspectDeliveryDispatch(context.Background(), request)
		requireOrcaCode(t, err, "runtime_error")
	})
	t.Run("dispatch inventory from another runtime", func(t *testing.T) {
		client := fx.fake()
		other := "runtime-other"
		client.dispatchInventoryRuntime = &other
		client.dispatch = dispatch
		_, exists, err := NewExecutionClient(client).InspectDeliveryDispatch(context.Background(), request)
		if err == nil || exists || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("exists=%t err=%v", exists, err)
		}
	})
	t.Run("no dispatch yet", func(t *testing.T) {
		receipt, exists, err := NewExecutionClient(fx.fake()).InspectDeliveryDispatch(context.Background(), request)
		if err != nil || exists || receipt != (port.ExecutionOrcaIntentReceipt{}) {
			t.Fatalf("receipt=%#v exists=%t err=%v", receipt, exists, err)
		}
	})
}

func TestInspectIntentRequiresALiveClient(t *testing.T) {
	var unavailable *ExecutionProvisioner
	_, err := unavailable.InspectIntent(context.Background(), newStageFixture(t).request(port.ExecutionOrcaIntentWorktree))
	if err == nil || !strings.Contains(err.Error(), "Orca client is unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestInspectIntentSurfacesReadFailures(t *testing.T) {
	fx := newStageFixture(t)
	boom := &port.OrcaError{Code: "runtime_error", Detail: "boom"}
	tests := []struct {
		name   string
		stage  port.ExecutionOrcaIntentStage
		script func(*scriptedExecutionFake)
	}{
		{"worktree list", port.ExecutionOrcaIntentWorktree, func(f *scriptedExecutionFake) { f.listWorktreesErr = boom }},
		{"terminal list", port.ExecutionOrcaIntentTerminal, func(f *scriptedExecutionFake) { f.listTerminalsErr = boom }},
		{"run list", port.ExecutionOrcaIntentRun, func(f *scriptedExecutionFake) { f.listRunsErr = boom }},
		{"current run", port.ExecutionOrcaIntentRunBind, func(f *scriptedExecutionFake) { f.currentInvEr = boom }},
		{"task list", port.ExecutionOrcaIntentTask, func(f *scriptedExecutionFake) { f.tasksErr = boom }},
		{"dispatch show", port.ExecutionOrcaIntentDispatch, func(f *scriptedExecutionFake) { f.dispatchShowErr = boom }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &scriptedExecutionFake{executionFake: fx.fake()}
			tt.script(client)
			_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(tt.stage))
			if !errors.Is(err, boom) {
				t.Fatalf("error = %v, want the Orca failure unchanged", err)
			}
		})
	}
}

func TestInspectIntentWithoutInventoryAccessFailsClosed(t *testing.T) {
	fx := newStageFixture(t)
	for _, tt := range []struct {
		stage port.ExecutionOrcaIntentStage
		want  string
	}{
		{port.ExecutionOrcaIntentTerminal, "intent inventory is unavailable"},
		{port.ExecutionOrcaIntentRun, "Run inventory is unavailable"},
		{port.ExecutionOrcaIntentRunBind, "Run inventory is unavailable"},
		{port.ExecutionOrcaIntentTask, "intent inventory is unavailable"},
		{port.ExecutionOrcaIntentDispatch, "intent inventory is unavailable"},
	} {
		t.Run(string(tt.stage), func(t *testing.T) {
			_, err := NewExecutionClient(requestOnlyClient{fx.fake()}).InspectIntent(context.Background(), fx.request(tt.stage))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestInspectIntentIgnoresUnrelatedRowsAndRejectsCorruptMatches(t *testing.T) {
	fx := newStageFixture(t)
	marker := fx.probe.Marker
	title := executionTaskTitle(marker, fx.launch.PromptSHA256)

	t.Run("worktree rows for other worktrees are skipped", func(t *testing.T) {
		other := executionWorktree(fx.workspace, fx.probe)
		other.Path, other.Comment, other.ID = "/elsewhere/wt", "someone else's marker", "wt-other"
		client := fx.fake()
		client.worktrees = []port.OrcaWorktree{other}
		inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentWorktree))
		if err != nil || !inventory.AuthoritativeZero || len(inventory.Candidates) != 0 {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
	t.Run("terminal rows with other titles are skipped", func(t *testing.T) {
		client := fx.fake()
		client.terminals = []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-x", PTYID: "pty-x", WorktreeID: "wt-69", Title: "shell", Connected: true, Writable: true}}
		inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTerminal))
		if err != nil || !inventory.AuthoritativeZero {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
	t.Run("terminal with the marker but a foreign worktree is rejected", func(t *testing.T) {
		client := fx.fake()
		client.terminals = []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-x", PTYID: "pty-x", WorktreeID: "wt-foreign", Title: marker, Connected: true, Writable: true}}
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTerminal))
		if err == nil || !strings.Contains(err.Error(), "worktree mismatch") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("run rows with other objectives are skipped", func(t *testing.T) {
		client := fx.fake()
		client.runs = []port.OrcaRun{{RuntimeID: "runtime-69", ID: "run-x", Objective: "another objective"}}
		inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRun))
		if err != nil || !inventory.AuthoritativeZero {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
	t.Run("run with the marker from another runtime is rejected", func(t *testing.T) {
		client := fx.fake()
		client.runs = []port.OrcaRun{{RuntimeID: "runtime-other", ID: "run-x", Objective: marker}}
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRun))
		if err == nil || !strings.Contains(err.Error(), "Run does not match the sealed runtime") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("run inventory from another runtime is rejected", func(t *testing.T) {
		client := &runtimeShiftedInventory{executionFake: fx.fake()}
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRun))
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("current run bind rejects a mismatching objective", func(t *testing.T) {
		client := fx.fake()
		client.currentRun = &port.OrcaRun{RuntimeID: "runtime-69", ID: "run-69", Objective: "different"}
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRunBind))
		if err == nil || !strings.Contains(err.Error(), "Run does not match the sealed runtime") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("current run bind reports the bound run", func(t *testing.T) {
		client := fx.fake()
		client.currentRun = &port.OrcaRun{RuntimeID: "runtime-69", ID: "run-69", Objective: marker}
		inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRunBind))
		if err != nil || len(inventory.Candidates) != 1 || inventory.Candidates[0].RunID != "run-69" || !inventory.Candidates[0].RunBound || inventory.AuthoritativeZero {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
	t.Run("no current run is authoritative zero", func(t *testing.T) {
		inventory, err := NewExecutionClient(fx.fake()).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRunBind))
		if err != nil || !inventory.AuthoritativeZero {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
	t.Run("current run inventory from another runtime is rejected", func(t *testing.T) {
		client := &runtimeShiftedInventory{executionFake: fx.fake()}
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentRunBind))
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("task rows with other titles are skipped", func(t *testing.T) {
		client := fx.fake()
		client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-x", Title: "unrelated", DisplayName: fx.workspace.Branch, Status: "ready"}}
		inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTask))
		if err != nil || !inventory.AuthoritativeZero {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
	t.Run("sealed task title with another display name is rejected", func(t *testing.T) {
		client := fx.fake()
		client.tasks = []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-x", Title: title, DisplayName: "someone-else", Status: "ready"}}
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTask))
		if err == nil || !strings.Contains(err.Error(), "task candidate does not match the sealed intent") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("task inventory from another runtime is rejected", func(t *testing.T) {
		client := fx.fake()
		other := "runtime-other"
		client.taskInventoryRuntime = &other
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), fx.request(port.ExecutionOrcaIntentTask))
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
}

// runtimeShiftedInventory answers Run reads from a runtime the seal never saw.
type runtimeShiftedInventory struct{ *executionFake }

func (r *runtimeShiftedInventory) listRunsInventory(ctx context.Context) (executionRunInventory, error) {
	inventory, err := r.executionFake.listRunsInventory(ctx)
	inventory.RuntimeID = "runtime-other"
	return inventory, err
}

func (r *runtimeShiftedInventory) currentRunInventory(ctx context.Context) (executionCurrentRunInventory, error) {
	inventory, err := r.executionFake.currentRunInventory(ctx)
	inventory.RuntimeID = "runtime-other"
	return inventory, err
}

func TestInspectIntentDispatchBranches(t *testing.T) {
	fx := newStageFixture(t)
	dispatch := port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69", Status: "dispatched", RequestID: stageDispatchRequestID}
	request := fx.request(port.ExecutionOrcaIntentDispatch)

	t.Run("inventory from another runtime", func(t *testing.T) {
		client := fx.fake()
		other := "runtime-other"
		client.dispatchInventoryRuntime = &other
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("a dispatch candidate needs a durable request id", func(t *testing.T) {
		client := fx.fake()
		client.dispatch = dispatch
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "requires a durable request ID") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("owner terminal missing", func(t *testing.T) {
		client := fx.fake()
		client.dispatch = dispatch
		client.terminals = nil
		pinned := request
		pinned.RetryRequestID = stageDispatchRequestID
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), pinned)
		if err == nil || !strings.Contains(err.Error(), "owner terminal is absent") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("dispatch with another request id", func(t *testing.T) {
		client := fx.fake()
		client.dispatch = dispatch
		pinned := request
		pinned.RetryRequestID = "99999999-9999-4999-8999-999999999999"
		_, err := NewExecutionClient(client).InspectIntent(context.Background(), pinned)
		if err == nil || !strings.Contains(err.Error(), "request identity") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("matching dispatch becomes the single candidate", func(t *testing.T) {
		client := fx.fake()
		client.dispatch = dispatch
		pinned := request
		pinned.RetryRequestID = stageDispatchRequestID
		inventory, err := NewExecutionClient(client).InspectIntent(context.Background(), pinned)
		want := port.ExecutionOrcaIntentReceipt{TaskID: "task-69", DispatchID: "dispatch-69", RequestID: stageDispatchRequestID, TerminalPTYID: "pty-69", TerminalHandle: "term-69"}
		if err != nil || len(inventory.Candidates) != 1 || inventory.Candidates[0] != want || inventory.AuthoritativeZero {
			t.Fatalf("inventory=%#v err=%v", inventory, err)
		}
	})
}

func TestResolveIntentTerminalFailures(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentDispatch)
	row := fx.fake().terminals[0]

	t.Run("no inventory access", func(t *testing.T) {
		_, err := NewExecutionClient(requestOnlyClient{fx.fake()}).resolveIntentTerminal(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "intent inventory is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("inventory failure", func(t *testing.T) {
		client := &scriptedExecutionFake{executionFake: fx.fake(), listTerminalsErr: &port.OrcaError{Code: "runtime_error"}}
		_, err := NewExecutionClient(client).resolveIntentTerminal(context.Background(), request)
		requireOrcaCode(t, err, "runtime_error")
	})
	t.Run("inventory from another runtime", func(t *testing.T) {
		client := fx.fake()
		other := "runtime-other"
		client.terminalInventoryRuntime = &other
		_, err := NewExecutionClient(client).resolveIntentTerminal(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("two rows for the sealed pty", func(t *testing.T) {
		client := fx.fake()
		client.terminals = []port.OrcaTerminal{row, row}
		_, err := NewExecutionClient(client).resolveIntentTerminal(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "inventory is ambiguous") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("the sealed pty row is not writable", func(t *testing.T) {
		client := fx.fake()
		client.terminals[0].Writable = false
		_, err := NewExecutionClient(client).resolveIntentTerminal(context.Background(), request)
		if err == nil || !strings.Contains(err.Error(), "not writable") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("other pty rows are ignored", func(t *testing.T) {
		client := fx.fake()
		other := row
		other.PTYID, other.Handle = "pty-other", "term-other"
		client.terminals = []port.OrcaTerminal{other, row}
		got, err := NewExecutionClient(client).resolveIntentTerminal(context.Background(), request)
		if err != nil || got.Handle != "term-69" {
			t.Fatalf("terminal=%#v err=%v", got, err)
		}
	})
}

func TestInspectOwnerFailsClosedOnEachInventoryRead(t *testing.T) {
	boom := &port.OrcaError{Code: "runtime_error", Detail: "boom"}
	owner := port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", RunID: "run-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	}
	base := func() *executionFake {
		return &executionFake{
			terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Connected: true, Writable: true}},
			tasks:     []port.OrcaTask{{RuntimeID: "runtime-69", ID: "task-69", Status: "running"}},
			dispatch:  port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69", Status: "running"},
		}
	}
	t.Run("no inventory access", func(t *testing.T) {
		_, err := NewExecutionClient(requestOnlyClient{base()}).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "owner inventory is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
	for name, script := range map[string]func(*scriptedExecutionFake){
		"terminal list": func(f *scriptedExecutionFake) { f.listTerminalsErr = boom },
		"task list":     func(f *scriptedExecutionFake) { f.tasksErr = boom },
		"dispatch show": func(f *scriptedExecutionFake) { f.dispatchShowErr = boom },
	} {
		t.Run(name, func(t *testing.T) {
			client := &scriptedExecutionFake{executionFake: base()}
			script(client)
			_, err := NewExecutionClient(client).InspectOwner(context.Background(), owner)
			if !errors.Is(err, boom) {
				t.Fatalf("error = %v, want the Orca failure unchanged", err)
			}
		})
	}
	t.Run("terminal detail read is unavailable", func(t *testing.T) {
		fake := base()
		_, err := NewExecutionClient(inventoryOnlyClient{executionClient: fake, inventory: fake}).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "terminal detail inventory is unavailable") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("terminal detail from another runtime", func(t *testing.T) {
		fake := base()
		pane := 1
		fake.terminalDetail = &executionTerminalDetailInventory{RuntimeID: "runtime-other", Terminal: fake.terminals[0], PaneRuntimeID: &pane}
		_, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("task inventory from another runtime", func(t *testing.T) {
		fake := base()
		other := "runtime-other"
		fake.taskInventoryRuntime = &other
		_, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("two task rows for one id", func(t *testing.T) {
		fake := base()
		fake.tasks = append(fake.tasks, fake.tasks[0])
		_, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "task inventory is ambiguous") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("task row from another runtime", func(t *testing.T) {
		fake := base()
		fake.tasks[0].RuntimeID = "runtime-other"
		_, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "task runtime identity changed") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("neither task nor dispatch remains", func(t *testing.T) {
		fake := base()
		fake.tasks, fake.dispatch = nil, port.OrcaDispatch{}
		got, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err != nil || got.TaskStatus != "" || got.TaskLive || got.DispatchStatus != "" || !got.TerminalLive {
			t.Fatalf("inventory=%#v err=%v", got, err)
		}
	})
	t.Run("assignee terminal appears twice", func(t *testing.T) {
		fake := base()
		fake.terminals = append(fake.terminals, port.OrcaTerminal{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-dup", WorktreeID: "wt-69"})
		_, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "assignee inventory is ambiguous") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("assignee terminal from another runtime", func(t *testing.T) {
		fake := base()
		fake.terminals = append(fake.terminals, port.OrcaTerminal{RuntimeID: "runtime-other", Handle: "term-assignee", PTYID: "pty-x", WorktreeID: "wt-69"})
		fake.dispatch.AssigneeHandle = "term-assignee"
		_, err := NewExecutionClient(fake).InspectOwner(context.Background(), owner)
		if err == nil || !strings.Contains(err.Error(), "assignee inventory is ambiguous") {
			t.Fatalf("error = %v", err)
		}
	})
}
