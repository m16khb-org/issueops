package orca

import (
	"context"
	"testing"

	"issueops/internal/port"
)

func TestInspectOwnerReadsTheRequestedTaskAmongOtherRows(t *testing.T) {
	client := &executionFake{
		terminals: []port.OrcaTerminal{{RuntimeID: "runtime-69", Handle: "term-69", PTYID: "pty-69", WorktreeID: "wt-69", Connected: true, Writable: true}},
		tasks: []port.OrcaTask{
			{RuntimeID: "runtime-69", ID: "task-other", Status: "completed"},
			{RuntimeID: "runtime-69", ID: "task-69", Status: " Failed "},
		},
		dispatch: port.OrcaDispatch{RuntimeID: "runtime-69", ID: "dispatch-69", TaskID: "task-69", AssigneeHandle: "term-69", Status: " Completed "},
	}
	got, err := NewExecutionClient(client).InspectOwner(context.Background(), port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: "runtime-69", WorktreeID: "wt-69", RunID: "run-69", TaskID: "task-69", DispatchID: "dispatch-69", TerminalPTYID: "pty-69",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskStatus != "failed" || got.TaskLive || got.DispatchStatus != "completed" {
		t.Fatalf("owner inventory = %#v, want the normalized statuses of the requested task only", got)
	}
}
