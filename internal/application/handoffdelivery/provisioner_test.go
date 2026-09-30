package handoffdelivery

import (
	"context"
	"errors"
	"testing"
	"time"

	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type stoppedAudit struct {
	calls   []string
	failure error
}

func (a *stoppedAudit) Read() ([]model.IssueOpsHandoffDeliveryObservation, error) {
	a.calls = append(a.calls, "read")
	return nil, nil
}
func (a *stoppedAudit) ReadFor(string, string) ([]model.IssueOpsHandoffDeliveryObservation, error) {
	a.calls = append(a.calls, "read_for")
	return nil, nil
}
func (a *stoppedAudit) Append(model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	a.calls = append(a.calls, "append")
	return auditcontract.HandoffDeliveryAuditRecord{}, a.failure
}

type stoppedExternal struct{ invokes int }

func (*stoppedExternal) Probe(context.Context, port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	return port.ExecutionOrcaProbeResult{}, nil
}
func (*stoppedExternal) InspectIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	return port.ExecutionOrcaIntentInventory{}, nil
}
func (e *stoppedExternal) InvokeIntent(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, error) {
	e.invokes++
	return port.ExecutionOrcaIntentReceipt{}, nil
}
func (*stoppedExternal) InspectDeliveryIdentity(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaDeliveryIdentity, error) {
	return port.ExecutionOrcaDeliveryIdentity{}, nil
}
func (*stoppedExternal) InspectDeliveryDispatch(context.Context, port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, bool, error) {
	return port.ExecutionOrcaIntentReceipt{}, false, nil
}
func (*stoppedExternal) ObserveRequest(context.Context, string) (port.OrcaRequestObservation, error) {
	return port.OrcaRequestObservation{}, nil
}

func TestDeliveryAuditFailurePreventsExternalInvoke(t *testing.T) {
	expected := errors.New("append unavailable")
	audit := &stoppedAudit{failure: expected}
	external := &stoppedExternal{}
	clocks := 0
	provisioner := Provisioner{Audit: audit, Next: external, Now: func() time.Time { clocks++; return time.Date(2026, 9, 29, 1, 0, 0, 0, time.UTC) }}
	_, err := provisioner.InvokeIntent(context.Background(), port.ExecutionOrcaIntentRequest{Stage: port.ExecutionOrcaIntentDispatch, Launch: &port.ExecutionOrcaLaunchRequest{}, Probe: port.ExecutionOrcaProbeRequest{Host: "codex"}})
	if !errors.Is(err, expected) || external.invokes != 0 || clocks != 1 {
		t.Fatalf("error=%v invokes=%d clocks=%d", err, external.invokes, clocks)
	}
	if len(audit.calls) != 5 || audit.calls[4] != "append" {
		t.Fatalf("audit order=%v", audit.calls)
	}
}
