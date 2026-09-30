package issueopslease

import (
	issueopscontract "issueops/internal/contract/issueops"
	issueopsport "issueops/internal/port"
)

import (
	"context"
	"errors"
	"testing"
)

func TestReconcileHandlerFailsClosedWithoutService(t *testing.T) {
	result, err := NewReconcileHandler(nil)(context.Background(), "state", issueopscontract.ExecutionReconcileRequest{ID: "io-reconcile"}, issueopsport.ExecutionReconcileDependencies{})
	if !errors.Is(err, issueopscontract.ErrReconcileHandlerUnavailable) || result.ID != "io-reconcile" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
