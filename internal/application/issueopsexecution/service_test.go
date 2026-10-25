package issueopsexecution

import (
	"context"
	"reflect"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestServiceStatusDoesNotReadRecord(t *testing.T) {
	calls := 0
	service := Service{ReadRecord: func(string, string) (model.IssueOpsRecord, error) {
		t.Fatal("status read record")
		return model.IssueOpsRecord{}, nil
	}}
	got, err := service.Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "cycle", Action: model.ExecutionActionStatus}, port.ExecutionActionDependencies{Status: func(_ context.Context, root, id string) (model.ExecutionResult, error) {
		calls++
		if root != "state" || id != "cycle" {
			t.Fatalf("scope=%s %s", root, id)
		}
		return model.ExecutionResult{OK: true, ID: id}, nil
	}})
	if err != nil || calls != 1 || !reflect.DeepEqual(got, model.ExecutionResult{OK: true, ID: "cycle"}) {
		t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
	}
}

func TestReconcileRejectsAmbiguousIntentBeforeObservations(t *testing.T) {
	service := Service{}
	for _, confirm := range []bool{false, true} {
		got, err := service.Reconcile(t.Context(), "state", model.ExecutionReconcileRequest{ID: "cycle", Preview: confirm, Confirm: confirm}, port.ExecutionReconcileDependencies{})
		if got.ID != "cycle" || got.OK || err == nil || !strings.HasPrefix(err.Error(), "execution reconcile requires exactly one of preview or confirm") {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	}
}
