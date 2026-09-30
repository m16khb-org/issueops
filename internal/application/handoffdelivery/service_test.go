package handoffdelivery

import (
	"errors"
	"reflect"
	"testing"

	auditcontract "issueops/internal/contract/audit"
	model "issueops/internal/contract/issueops"
)

type auditProbe struct {
	calls []string
	err   error
}

func (p *auditProbe) Append(o model.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error) {
	p.calls = append(p.calls, "append")
	return auditcontract.HandoffDeliveryAuditRecord{Observation: o}, p.err
}
func (p *auditProbe) Read() ([]model.IssueOpsHandoffDeliveryObservation, error) {
	p.calls = append(p.calls, "read")
	return nil, p.err
}
func (p *auditProbe) ReadFor(id, lineage string) ([]model.IssueOpsHandoffDeliveryObservation, error) {
	p.calls = append(p.calls, "read:"+id+":"+lineage)
	return nil, p.err
}

func TestManualObservationRefusalHasNoAppendAndPreservesReadError(t *testing.T) {
	audit := &auditProbe{}
	readErr := errors.New("record unavailable")
	service := Service{Audit: audit, ReadRecord: func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, readErr }}
	if _, err := service.ObserveManual(model.IssueOpsHandoffDeliveryObservation{}); err != readErr {
		t.Fatalf("read error replaced: %v", err)
	}
	service.ReadRecord = func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, nil }
	if _, err := service.ObserveManual(model.IssueOpsHandoffDeliveryObservation{}); err == nil || err.Error() != "manual handoff delivery observation requires the exact released direct execution generation" {
		t.Fatalf("authority error: %v", err)
	}
	if len(audit.calls) != 0 {
		t.Fatalf("refusal touched audit: %v", audit.calls)
	}
}

func TestClaimObservationChoosesScopedReadAndStopsOnReadError(t *testing.T) {
	for _, test := range []struct {
		name   string
		result model.ExecutionResult
		want   []string
	}{
		{name: "failed result"},
		{name: "missing holder", result: model.ExecutionResult{OK: true, Execution: model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Status: model.LeaseStatusActive}}}},
		{name: "orca scoped", result: model.ExecutionResult{OK: true, ID: "io-test", Execution: model.Execution{Orca: &model.OrcaBinding{OwnerHost: "omo", OwnerPromptSHA256: "prompt", ContextPacketSHA256: "material"}, Lease: model.WriteLease{Status: model.LeaseStatusActive, Generation: 2, Holder: &model.NativeActor{Host: "omo"}}}}, want: []string{"read:io-test:generation:2:prompt:prompt:material:material:call:prompt"}},
		{name: "direct", result: model.ExecutionResult{OK: true, ID: "io-test", Execution: model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Status: model.LeaseStatusActive, Generation: 2, Holder: &model.NativeActor{Host: "codex"}}}}, want: []string{"read"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := &auditProbe{err: errors.New("unreadable audit")}
			s := Service{Audit: p}
			err := s.ObserveClaim(test.result)
			if len(test.want) > 0 && err != p.err {
				t.Fatalf("error=%v", err)
			}
			if len(test.want) == 0 && err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(p.calls, test.want) {
				t.Fatalf("calls=%v want=%v", p.calls, test.want)
			}
		})
	}
}
