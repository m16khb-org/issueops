package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	preparation "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

type fakeIntentOrca struct {
	port.ExecutionOrcaProvisioner
	requests []port.ExecutionOrcaIntentRequest
	inspect  func(call int, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error)
}

func (f *fakeIntentOrca) InspectIntent(_ context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	f.requests = append(f.requests, req)
	if f.inspect == nil {
		return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
	}
	return f.inspect(len(f.requests), req)
}

type fakeOwnerInspector struct {
	requests []port.ExecutionOrcaOwnerInventoryRequest
	result   port.ExecutionOrcaOwnerInventory
	err      error
}

func (f *fakeOwnerInspector) InspectOwner(_ context.Context, req port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	f.requests = append(f.requests, req)
	return f.result, f.err
}

func pendingRecord(kind string) model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "cycle", Execution: &model.Execution{
		Mode:    model.ExecutionModeOrca,
		Lease:   model.WriteLease{Generation: 4},
		Pending: &model.ExternalIntent{OperationID: "op-1", Kind: kind, Marker: "marker-1"},
	}}
}

func matchingIntent(stage preparation.IntentStage) preparation.Intent {
	return preparation.Intent{OperationID: "op-1", LifecycleID: "cycle", Generation: 4, Marker: "marker-1", Stage: stage}
}

func sealedRequest() port.ExecutionOrcaIntentRequest {
	return port.ExecutionOrcaIntentRequest{
		Marker: "marker-1", Prepared: &port.ExecutionOrcaWorkspaceReceipt{}, Launch: &port.ExecutionOrcaLaunchRequest{},
		TerminalPTYID: "pty-1", RunID: "run-1", RunBound: false, TaskID: "task-1",
	}
}

func observerFor(intent preparation.Intent, orca port.ExecutionOrcaProvisioner) AbandonOrcaObserver {
	return AbandonOrcaObserver{
		ReadIntent: func(operationID string) (preparation.Intent, error) {
			if operationID != "op-1" {
				return preparation.Intent{}, errors.New("unexpected operation " + operationID)
			}
			return intent, nil
		},
		InspectionRequest: func(model.IssueOpsRecord, preparation.Intent) (port.ExecutionOrcaIntentRequest, error) {
			return sealedRequest(), nil
		},
		Orca: orca,
	}
}

func TestAbandonOrcaPendingSafeRefusesUnsafePendingIntents(t *testing.T) {
	readFailure, requestFailure := errors.New("intent row missing"), errors.New("cannot seal request")
	cases := []struct {
		name            string
		record          model.IssueOpsRecord
		worktreePresent bool
		observer        func(AbandonOrcaObserver) AbandonOrcaObserver
		want            string
		wantErr         error
	}{
		{name: "non-local pending kind", record: pendingRecord("reconcile"), want: "is not local-only"},
		{name: "non-orca execution", record: func() model.IssueOpsRecord {
			r := pendingRecord("worktree_create")
			r.Execution.Mode = model.ExecutionModeDirect
			return r
		}(), want: "requires an Orca execution record"},
		{name: "worktree still on disk", record: pendingRecord("worktree_create"), worktreePresent: true, want: "still exists on disk"},
		{name: "intent row unreadable", record: pendingRecord("worktree_create"), observer: func(o AbandonOrcaObserver) AbandonOrcaObserver {
			o.ReadIntent = func(string) (preparation.Intent, error) { return preparation.Intent{}, readFailure }
			return o
		}, wantErr: readFailure},
		{name: "intent belongs to another lifecycle", record: pendingRecord("worktree_create"), observer: func(o AbandonOrcaObserver) AbandonOrcaObserver {
			o.ReadIntent = func(string) (preparation.Intent, error) {
				intent := matchingIntent(preparation.IntentStageWorktree)
				intent.LifecycleID = "other"
				return intent, nil
			}
			return o
		}, want: "does not belong to this lifecycle"},
		{name: "intent generation is stale", record: pendingRecord("worktree_create"), observer: func(o AbandonOrcaObserver) AbandonOrcaObserver {
			o.ReadIntent = func(string) (preparation.Intent, error) {
				intent := matchingIntent(preparation.IntentStageWorktree)
				intent.Generation = 3
				return intent, nil
			}
			return o
		}, want: "does not belong to this lifecycle"},
		{name: "intent stage disagrees with pending kind", record: pendingRecord("worktree_create"), observer: func(o AbandonOrcaObserver) AbandonOrcaObserver {
			o.ReadIntent = func(string) (preparation.Intent, error) { return matchingIntent(preparation.IntentStageDispatch), nil }
			return o
		}, want: "does not belong to this lifecycle"},
		{name: "inspector not configured", record: pendingRecord("worktree_create"), observer: func(o AbandonOrcaObserver) AbandonOrcaObserver {
			o.Orca = nil
			return o
		}, want: "Orca intent inspector is not configured"},
		{name: "sealed request unavailable", record: pendingRecord("worktree_create"), observer: func(o AbandonOrcaObserver) AbandonOrcaObserver {
			o.InspectionRequest = func(model.IssueOpsRecord, preparation.Intent) (port.ExecutionOrcaIntentRequest, error) {
				return port.ExecutionOrcaIntentRequest{}, requestFailure
			}
			return o
		}, wantErr: requestFailure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orca := &fakeIntentOrca{}
			observer := observerFor(matchingIntent(preparation.IntentStageWorktree), orca)
			if tc.observer != nil {
				observer = tc.observer(observer)
			}
			err := observer.PendingSafe(t.Context(), tc.record, tc.worktreePresent)
			if err == nil {
				t.Fatal("PendingSafe accepted an unsafe pending intent")
			}
			if tc.want != "" && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v, want wrapping %v", err, tc.wantErr)
			}
			if len(orca.requests) != 0 {
				t.Fatalf("Orca inspected despite refusal: %v", orca.requests)
			}
		})
	}
}

func TestAbandonOrcaPendingSafeInspectsEveryResourceUpToTheSealedStage(t *testing.T) {
	cases := []struct {
		stage      preparation.IntentStage
		kind       string
		wantStages []port.ExecutionOrcaIntentStage
	}{
		{preparation.IntentStageWorktree, "worktree_create", []port.ExecutionOrcaIntentStage{port.ExecutionOrcaIntentWorktree}},
		{preparation.IntentStageTerminal, "owner_launch", []port.ExecutionOrcaIntentStage{port.ExecutionOrcaIntentWorktree, port.ExecutionOrcaIntentTerminal}},
		{preparation.IntentStageRun, "owner_launch", []port.ExecutionOrcaIntentStage{port.ExecutionOrcaIntentWorktree, port.ExecutionOrcaIntentTerminal}},
		{preparation.IntentStageRunBind, "owner_launch", []port.ExecutionOrcaIntentStage{port.ExecutionOrcaIntentWorktree, port.ExecutionOrcaIntentTerminal}},
		{preparation.IntentStageTask, "owner_launch", []port.ExecutionOrcaIntentStage{port.ExecutionOrcaIntentWorktree, port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentTask}},
		{preparation.IntentStageDispatch, "dispatch", []port.ExecutionOrcaIntentStage{port.ExecutionOrcaIntentWorktree, port.ExecutionOrcaIntentTerminal, port.ExecutionOrcaIntentTask, port.ExecutionOrcaIntentDispatch}},
	}
	for _, tc := range cases {
		t.Run(string(tc.stage), func(t *testing.T) {
			orca := &fakeIntentOrca{}
			if err := observerFor(matchingIntent(tc.stage), orca).PendingSafe(t.Context(), pendingRecord(tc.kind), false); err != nil {
				t.Fatalf("PendingSafe: %v", err)
			}
			var got []port.ExecutionOrcaIntentStage
			for _, req := range orca.requests {
				got = append(got, req.Stage)
			}
			if !slices.Equal(got, tc.wantStages) {
				t.Fatalf("inspected stages %v, want %v", got, tc.wantStages)
			}
		})
	}
}

func TestAbandonOrcaPendingSafeProjectsRequestPerStageWithoutLaterResources(t *testing.T) {
	orca := &fakeIntentOrca{}
	if err := observerFor(matchingIntent(preparation.IntentStageDispatch), orca).PendingSafe(t.Context(), pendingRecord("dispatch"), false); err != nil {
		t.Fatal(err)
	}
	worktree, terminal, task, dispatch := orca.requests[0], orca.requests[1], orca.requests[2], orca.requests[3]
	if worktree.Prepared != nil || worktree.Launch != nil || worktree.TerminalPTYID != "" || worktree.RunID != "" || worktree.RunBound || worktree.TaskID != "" {
		t.Fatalf("worktree stage inherited later resources: %+v", worktree)
	}
	if terminal.Prepared == nil || terminal.Launch == nil || terminal.TerminalPTYID != "" || terminal.RunID != "" || terminal.RunBound || terminal.TaskID != "" {
		t.Fatalf("terminal stage must keep the launch receipts but not terminal/run/task ids: %+v", terminal)
	}
	if task.TerminalPTYID != "pty-1" || task.RunID != "run-1" || !task.RunBound || task.TaskID != "" {
		t.Fatalf("task stage must be run-bound with the task id still unknown: %+v", task)
	}
	if dispatch.TaskID != "task-1" || dispatch.TerminalPTYID != "pty-1" || dispatch.RunID != "run-1" || dispatch.RunBound {
		t.Fatalf("dispatch stage must be passed through unchanged: %+v", dispatch)
	}
	if dispatch.Marker != "marker-1" {
		t.Fatalf("marker lost: %+v", dispatch)
	}
}

func TestAbandonOrcaPendingSafeRejectsAmbiguousInventoryAndStopsAtFirstProblem(t *testing.T) {
	inspectFailure := errors.New("orca unreachable")
	cases := []struct {
		name      string
		result    port.ExecutionOrcaIntentInventory
		err       error
		want      string
		wantErrIs error
	}{
		{name: "inventory error", err: inspectFailure, want: "worktree_create intent inventory is ambiguous", wantErrIs: inspectFailure},
		{name: "candidate present", result: port.ExecutionOrcaIntentInventory{Candidates: make([]port.ExecutionOrcaIntentReceipt, 2), AuthoritativeZero: true}, want: "found 2 candidate(s)"},
		{name: "non-authoritative zero", result: port.ExecutionOrcaIntentInventory{}, want: "non-authoritative zero"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orca := &fakeIntentOrca{inspect: func(int, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
				return tc.result, tc.err
			}}
			err := observerFor(matchingIntent(preparation.IntentStageDispatch), orca).PendingSafe(t.Context(), pendingRecord("dispatch"), false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
			if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
				t.Fatalf("err=%v does not wrap %v", err, tc.wantErrIs)
			}
			if len(orca.requests) != 1 {
				t.Fatalf("inspected %d stages after the first failure, want 1", len(orca.requests))
			}
		})
	}
	t.Run("later stage candidate", func(t *testing.T) {
		orca := &fakeIntentOrca{inspect: func(call int, _ port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
			if call == 2 {
				return port.ExecutionOrcaIntentInventory{Candidates: make([]port.ExecutionOrcaIntentReceipt, 1)}, nil
			}
			return port.ExecutionOrcaIntentInventory{AuthoritativeZero: true}, nil
		}}
		err := observerFor(matchingIntent(preparation.IntentStageTask), orca).PendingSafe(t.Context(), pendingRecord("owner_launch"), false)
		if err == nil || !strings.Contains(err.Error(), "terminal_create intent inventory found 1") || len(orca.requests) != 2 {
			t.Fatalf("err=%v inspected=%d", err, len(orca.requests))
		}
	})
}

func orcaBoundRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{ID: "cycle", Execution: &model.Execution{
		Mode: model.ExecutionModeOrca,
		Orca: &model.OrcaBinding{RuntimeID: "rt", WorktreeID: "wt", RunID: "run", TaskID: "task", DispatchID: "disp", TerminalPTYID: "pty"},
	}}
}

func TestAbandonOrcaResourcesAbsentSkipsCyclesWithoutALiveTaskBinding(t *testing.T) {
	noTask := orcaBoundRecord()
	noTask.Execution.Orca.TaskID = "  "
	direct := orcaBoundRecord()
	direct.Execution.Mode = model.ExecutionModeDirect
	unbound := orcaBoundRecord()
	unbound.Execution.Orca = nil
	for name, record := range map[string]model.IssueOpsRecord{"no task id": noTask, "direct mode": direct, "no orca binding": unbound, "no execution": {ID: "cycle"}} {
		owner := &fakeOwnerInspector{err: errors.New("must not be called")}
		if err := (AbandonOrcaObserver{Owner: owner}).ResourcesAbsent(t.Context(), record, true, false); err != nil || len(owner.requests) != 0 {
			t.Fatalf("%s: err=%v requests=%v", name, err, owner.requests)
		}
	}
}

func TestAbandonOrcaResourcesAbsentJudgesOwnerInventory(t *testing.T) {
	cases := []struct {
		name      string
		observer  func() AbandonOrcaObserver
		reachable bool
		want      string
	}{
		{name: "inspector missing", observer: func() AbandonOrcaObserver { return AbandonOrcaObserver{} }, want: "owner inspector is not configured"},
		{name: "inventory ambiguous", observer: func() AbandonOrcaObserver {
			return AbandonOrcaObserver{Owner: &fakeOwnerInspector{err: errors.New("rpc")}}
		}, want: "owner inventory is ambiguous"},
		{name: "task still live", observer: func() AbandonOrcaObserver {
			return AbandonOrcaObserver{Owner: &fakeOwnerInspector{result: port.ExecutionOrcaOwnerInventory{TaskLive: true, TaskStatus: "running"}}}
		}, reachable: true, want: `task_status="running"`},
		{name: "terminal live and not stoppable", observer: func() AbandonOrcaObserver {
			return AbandonOrcaObserver{Owner: &fakeOwnerInspector{result: port.ExecutionOrcaOwnerInventory{TerminalLive: true}}}
		}, want: "terminal_live=true terminal_reachable_by_stop=false"},
		{name: "terminal live but stoppable", observer: func() AbandonOrcaObserver {
			return AbandonOrcaObserver{Owner: &fakeOwnerInspector{result: port.ExecutionOrcaOwnerInventory{TerminalLive: true}}}
		}, reachable: true},
		{name: "nothing live", observer: func() AbandonOrcaObserver { return AbandonOrcaObserver{Owner: &fakeOwnerInspector{}} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.observer().ResourcesAbsent(t.Context(), orcaBoundRecord(), false, tc.reachable)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("err=%v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestAbandonOrcaResourcesAbsentOnlyAllowsRuntimeRolloverForHolderlessLease(t *testing.T) {
	for _, tc := range []struct {
		name       string
		holderless bool
		holder     *model.NativeActor
		want       bool
	}{
		{name: "holderless lease without holder", holderless: true, want: true},
		{name: "holderless flag but recorded holder", holderless: true, holder: &model.NativeActor{Host: "codex"}},
		{name: "lease not holderless", holder: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := orcaBoundRecord()
			record.Execution.Lease.Holder = tc.holder
			owner := &fakeOwnerInspector{}
			if err := (AbandonOrcaObserver{Owner: owner}).ResourcesAbsent(t.Context(), record, tc.holderless, false); err != nil {
				t.Fatal(err)
			}
			want := port.ExecutionOrcaOwnerInventoryRequest{RuntimeID: "rt", WorktreeID: "wt", RunID: "run", TaskID: "task", DispatchID: "disp", TerminalPTYID: "pty", AllowRuntimeRollover: tc.want}
			if len(owner.requests) != 1 || owner.requests[0] != want {
				t.Fatalf("requests=%+v, want %+v", owner.requests, want)
			}
		})
	}
}
