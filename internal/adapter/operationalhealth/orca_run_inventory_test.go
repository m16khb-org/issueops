package operationalhealth

import (
	stdcontext "context"
	"errors"
	"reflect"
	"testing"
	"time"

	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	corehealth "issueops/internal/domain/operationalhealth"
	"issueops/internal/port"
)

func orcaRunFixture() *inventoryOrca {
	return &inventoryOrca{
		repo:  port.OrcaRepo{RuntimeID: "runtime", ID: "repo", Path: "/repo"},
		inbox: port.OrcaInboxPresence{RuntimeID: "runtime"},
		runInventoryOrca: runInventoryOrca{inventory: port.OrcaRunInventory{
			RuntimeID: "runtime",
			Runs:      []port.OrcaRun{{RuntimeID: "runtime", ID: "run-1"}},
		}},
	}
}

func collectOrcaFixture(orca OrcaInventory, owned bool) corehealth.Snapshot {
	snapshot := corehealth.Snapshot{RepoRoot: "/repo", Messages: operationalhealthcontract.MessagePresence{Empty: true}}
	Collector{Orca: orca}.collectOrca(stdcontext.Background(), &snapshot, owned)
	return snapshot
}

func TestCollectOrcaUnavailableIsOnlyAProblemWhenOrcaOwnsRecords(t *testing.T) {
	snapshot := collectOrcaFixture(nil, false)
	if len(snapshot.InventoryProblems) != 0 || !snapshot.Messages.CompleteAbsence {
		t.Fatalf("unowned: %+v", snapshot)
	}
	snapshot = collectOrcaFixture(nil, true)
	if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"orca_unavailable"}) || snapshot.Messages.CompleteAbsence {
		t.Fatalf("owned: problems=%v messages=%+v", got, snapshot.Messages)
	}
}

func TestCollectOrcaReportsRunInventoryFailureAndIdentityMismatch(t *testing.T) {
	failing := &failingRunsOrca{inventoryOrca: *orcaRunFixture()}
	snapshot := collectOrcaFixture(failing, false)
	if !hasProblemCode(snapshot.InventoryProblems, "orca_runs_failed") || len(snapshot.Tasks) != 0 {
		t.Fatalf("problems=%v tasks=%v", problemCodes(snapshot.InventoryProblems), snapshot.Tasks)
	}

	for name, run := range map[string]port.OrcaRun{
		"foreign runtime": {RuntimeID: "other", ID: "run-1"},
		"blank id":        {RuntimeID: "runtime", ID: "  "},
	} {
		orca := orcaRunFixture()
		orca.inventory.Runs = []port.OrcaRun{{RuntimeID: "runtime", ID: "ok"}, run}
		orca.allTasks = []port.OrcaTask{{RuntimeID: "runtime", ID: "t"}}
		snapshot := collectOrcaFixture(orca, false)
		if !hasProblemCode(snapshot.InventoryProblems, "orca_run_identity_mismatch") || len(orca.readerInventories) != 0 || len(snapshot.Tasks) != 0 {
			t.Errorf("%s: problems=%v readers=%d tasks=%v", name, problemCodes(snapshot.InventoryProblems), len(orca.readerInventories), snapshot.Tasks)
		}
	}
}

type failingRunsOrca struct{ inventoryOrca }

func (*failingRunsOrca) ListRunInventory(_ stdcontext.Context) (port.OrcaRunInventory, error) {
	return port.OrcaRunInventory{}, errors.New("runs unavailable")
}

func TestApplyOrcaTasksNormalizesRowsAndFlagsForeignRuntimeAndBadTimestamps(t *testing.T) {
	orca := orcaRunFixture()
	orca.allTasks = []port.OrcaTask{
		{RuntimeID: " runtime ", RunID: " run-1 ", ID: " t1 ", Status: " completed ", CompletedAt: " 2026-03-04T05:06:07Z ", HasResult: true},
		{RuntimeID: "other", RunID: "run-1", ID: "t2", Status: "ready", CompletedAt: "yesterday"},
		{RuntimeID: "runtime", RunID: "run-1", ID: "t3", Status: "ready", CompletedAt: "   "},
	}
	snapshot := collectOrcaFixture(orca, false)

	want := []operationalhealthcontract.OrcaTask{
		{RuntimeID: "runtime", RunID: "run-1", ID: "t1", Status: "completed", CompletedAt: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC), HasResult: true},
		{RuntimeID: "other", RunID: "run-1", ID: "t2", Status: "ready"},
		{RuntimeID: "runtime", RunID: "run-1", ID: "t3", Status: "ready"},
	}
	if !reflect.DeepEqual(snapshot.Tasks, want) {
		t.Fatalf("tasks = %+v\nwant   %+v", snapshot.Tasks, want)
	}
	got := problemCodes(snapshot.InventoryProblems)
	if !reflect.DeepEqual(got, []string{"orca_task_runtime_mismatch", "orca_task_timestamp_invalid"}) {
		t.Fatalf("problems = %v", got)
	}
}

func TestApplyOrcaDispatchesBindsDispatchIDToTaskAndValidatesIdentity(t *testing.T) {
	orca := orcaRunFixture()
	task := port.OrcaTask{RuntimeID: "runtime", RunID: "run-1", ID: "t1", Status: "dispatched"}
	orca.allTasks = []port.OrcaTask{task}
	orca.dispatched = []port.OrcaTask{task}
	orca.dispatch = port.OrcaDispatch{RuntimeID: "runtime", ID: " d1 ", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"}
	snapshot := collectOrcaFixture(orca, false)

	if len(snapshot.InventoryProblems) != 0 {
		t.Fatalf("problems = %v", snapshot.InventoryProblems)
	}
	if len(snapshot.Dispatches) != 1 || snapshot.Dispatches[0] != (operationalhealthcontract.OrcaDispatch{RuntimeID: "runtime", RunID: "run-1", ID: "d1", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"}) {
		t.Fatalf("dispatches = %+v", snapshot.Dispatches)
	}
	if snapshot.Tasks[0].DispatchID != "d1" {
		t.Fatalf("task must carry its dispatch id: %+v", snapshot.Tasks[0])
	}

	for name, dispatch := range map[string]port.OrcaDispatch{
		"foreign runtime": {RuntimeID: "other", ID: "d", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"},
		"missing id":      {RuntimeID: "runtime", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"},
		"other task":      {RuntimeID: "runtime", ID: "d", TaskID: "t9", AssigneeHandle: "h", Status: "dispatched"},
		"no assignee":     {RuntimeID: "runtime", ID: "d", TaskID: "t1", Status: "dispatched"},
		"not dispatched":  {RuntimeID: "runtime", ID: "d", TaskID: "t1", AssigneeHandle: "h", Status: "queued"},
	} {
		orca.dispatch = dispatch
		snapshot := collectOrcaFixture(orca, false)
		if !hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_identity_mismatch") {
			t.Errorf("%s: problems=%v", name, problemCodes(snapshot.InventoryProblems))
		}
	}
	orca.dispatch = port.OrcaDispatch{RuntimeID: "runtime", ID: "d", TaskID: "t9", AssigneeHandle: "h", Status: "dispatched"}
	if snapshot := collectOrcaFixture(orca, false); snapshot.Tasks[0].DispatchID != "" {
		t.Errorf("a dispatch for another task must not be bound: %+v", snapshot.Tasks[0])
	}
}

func TestApplyOrcaDispatchesDetectsSetMismatchAmbiguityAndShowFailure(t *testing.T) {
	dispatchedTask := port.OrcaTask{RuntimeID: "runtime", RunID: "run-1", ID: "t1", Status: "dispatched"}

	t.Run("dispatched task absent from dispatched set", func(t *testing.T) {
		orca := orcaRunFixture()
		orca.allTasks = []port.OrcaTask{dispatchedTask}
		snapshot := collectOrcaFixture(orca, false)
		if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"orca_dispatch_task_mismatch"}) {
			t.Fatalf("problems = %v", got)
		}
	})
	t.Run("duplicate dispatched rows are ambiguous", func(t *testing.T) {
		orca := orcaRunFixture()
		orca.allTasks = []port.OrcaTask{dispatchedTask}
		orca.dispatched = []port.OrcaTask{dispatchedTask, dispatchedTask}
		orca.dispatchErr = errors.New("show dispatch down")
		snapshot := collectOrcaFixture(orca, false)
		if !hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_task_mismatch") || !hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_failed") {
			t.Fatalf("problems = %v", problemCodes(snapshot.InventoryProblems))
		}
		if len(snapshot.Dispatches) != 0 {
			t.Fatalf("failed ShowDispatch must not add a dispatch: %+v", snapshot.Dispatches)
		}
	})
	t.Run("blank task id is ambiguous and never shown", func(t *testing.T) {
		orca := orcaRunFixture()
		orca.dispatched = []port.OrcaTask{{RuntimeID: "runtime", RunID: "run-1", ID: "  ", Status: "dispatched"}}
		orca.dispatch = port.OrcaDispatch{RuntimeID: "runtime", ID: "d", TaskID: "x", AssigneeHandle: "h", Status: "dispatched"}
		snapshot := collectOrcaFixture(orca, false)
		if got := problemCodes(snapshot.InventoryProblems); !reflect.DeepEqual(got, []string{"orca_dispatch_task_mismatch"}) || len(snapshot.Dispatches) != 0 {
			t.Fatalf("problems=%v dispatches=%+v", got, snapshot.Dispatches)
		}
	})
	t.Run("dispatched row whose task is not dispatched in the task list", func(t *testing.T) {
		orca := orcaRunFixture()
		orca.allTasks = []port.OrcaTask{{RuntimeID: "runtime", RunID: "run-1", ID: "t1", Status: "ready"}}
		orca.dispatched = []port.OrcaTask{dispatchedTask}
		orca.dispatch = port.OrcaDispatch{RuntimeID: "runtime", ID: "d", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"}
		snapshot := collectOrcaFixture(orca, false)
		if !hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_task_mismatch") {
			t.Fatalf("problems = %v", problemCodes(snapshot.InventoryProblems))
		}
	})
	t.Run("foreign runtime dispatched row", func(t *testing.T) {
		orca := orcaRunFixture()
		foreign := dispatchedTask
		foreign.RuntimeID = "other"
		orca.allTasks = []port.OrcaTask{dispatchedTask}
		orca.dispatched = []port.OrcaTask{foreign}
		orca.dispatch = port.OrcaDispatch{RuntimeID: "runtime", ID: "d", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"}
		snapshot := collectOrcaFixture(orca, false)
		if !hasProblemCode(snapshot.InventoryProblems, "orca_dispatched_task_runtime_mismatch") {
			t.Fatalf("problems = %v", problemCodes(snapshot.InventoryProblems))
		}
	})
	t.Run("same task id in different runs stays distinct", func(t *testing.T) {
		orca := orcaRunFixture()
		orca.inventory.Runs = append(orca.inventory.Runs, port.OrcaRun{RuntimeID: "runtime", ID: "run-2"})
		other := dispatchedTask
		other.RunID = "run-2"
		orca.allTasks = []port.OrcaTask{dispatchedTask, other}
		orca.dispatched = []port.OrcaTask{dispatchedTask, other}
		orca.dispatch = port.OrcaDispatch{RuntimeID: "runtime", ID: "d", TaskID: "t1", AssigneeHandle: "h", Status: "dispatched"}
		snapshot := collectOrcaFixture(orca, false)
		if hasProblemCode(snapshot.InventoryProblems, "orca_dispatch_task_mismatch") {
			t.Fatalf("per-run keys must not collide: %v", problemCodes(snapshot.InventoryProblems))
		}
		if len(snapshot.Dispatches) != 2 {
			t.Fatalf("dispatches = %+v", snapshot.Dispatches)
		}
	})
}

func TestApplyOrcaGatesNormalizesRowsAndFlagsForeignRuntime(t *testing.T) {
	orca := orcaRunFixture()
	orca.gates = []port.OrcaGate{
		{RuntimeID: " runtime", ID: " g1 ", TaskID: " t1 ", Status: " open "},
		{RuntimeID: "other", ID: "g2", TaskID: "t2", Status: "closed"},
	}
	snapshot := collectOrcaFixture(orca, false)
	want := []operationalhealthcontract.OrcaGate{
		{RuntimeID: "runtime", ID: "g1", TaskID: "t1", Status: "open"},
		{RuntimeID: "other", ID: "g2", TaskID: "t2", Status: "closed"},
	}
	if !reflect.DeepEqual(snapshot.Gates, want) || !reflect.DeepEqual(problemCodes(snapshot.InventoryProblems), []string{"orca_gate_runtime_mismatch"}) {
		t.Fatalf("gates=%+v problems=%v", snapshot.Gates, problemCodes(snapshot.InventoryProblems))
	}
}

func TestCollectOrcaInboxValidatesBoundedCountAgainstRows(t *testing.T) {
	cases := []struct {
		name     string
		inbox    port.OrcaInboxPresence
		err      error
		wantCode string
		want     operationalhealthcontract.MessagePresence
	}{
		{name: "empty", inbox: port.OrcaInboxPresence{RuntimeID: "runtime", CompleteAbsence: true}, want: operationalhealthcontract.MessagePresence{RuntimeID: "runtime", Empty: true, CompleteAbsence: true}},
		{name: "one row", inbox: port.OrcaInboxPresence{RuntimeID: "runtime", Count: 1, RowCount: 1}, want: operationalhealthcontract.MessagePresence{RuntimeID: "runtime", Count: 1}},
		{name: "count disagrees with rows", inbox: port.OrcaInboxPresence{RuntimeID: "runtime", Count: 3, RowCount: 1}, wantCode: "orca_inbox_count_mismatch", want: operationalhealthcontract.MessagePresence{RuntimeID: "runtime", Count: 3}},
		{name: "more rows than the bound", inbox: port.OrcaInboxPresence{RuntimeID: "runtime", Count: 2, RowCount: 2}, wantCode: "orca_inbox_count_mismatch", want: operationalhealthcontract.MessagePresence{RuntimeID: "runtime", Count: 2}},
		{name: "negative rows", inbox: port.OrcaInboxPresence{RuntimeID: "runtime", Count: -1, RowCount: -1}, wantCode: "orca_inbox_count_mismatch", want: operationalhealthcontract.MessagePresence{RuntimeID: "runtime", Count: -1, Empty: false}},
		{name: "foreign runtime", inbox: port.OrcaInboxPresence{RuntimeID: "other"}, wantCode: "orca_inbox_count_mismatch", want: operationalhealthcontract.MessagePresence{RuntimeID: "other", Empty: true}},
		{name: "read failure keeps default presence", err: errors.New("boom"), wantCode: "orca_inbox_failed", want: operationalhealthcontract.MessagePresence{Empty: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			orca := orcaRunFixture()
			orca.inbox, orca.inboxErr = c.inbox, c.err
			snapshot := collectOrcaFixture(orca, false)
			var got string
			if len(snapshot.InventoryProblems) > 0 {
				got = snapshot.InventoryProblems[0].Code
			}
			if got != c.wantCode || snapshot.Messages != c.want {
				t.Fatalf("code=%q messages=%+v, want %q %+v", got, snapshot.Messages, c.wantCode, c.want)
			}
		})
	}
}

func TestCollectOrcaStatusFailureIsOptionalUnlessOrcaOwnsRecords(t *testing.T) {
	for _, owned := range []bool{false, true} {
		orca := &statusFailOrca{}
		snapshot := collectOrcaFixture(orca, owned)
		hasProblem := hasProblemCode(snapshot.InventoryProblems, "orca_status_failed")
		if hasProblem != owned || snapshot.OrcaObserved || snapshot.Messages.CompleteAbsence == owned {
			t.Errorf("owned=%v: problems=%v observed=%v messages=%+v", owned, problemCodes(snapshot.InventoryProblems), snapshot.OrcaObserved, snapshot.Messages)
		}
	}
}

type statusFailOrca struct{ statusOnlyOrca }

func (*statusFailOrca) Available() bool { return true }
func (*statusFailOrca) Status(_ stdcontext.Context) (port.OrcaStatus, error) {
	return port.OrcaStatus{}, errors.New("status down")
}
