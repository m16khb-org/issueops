package issueopscleanup_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type auditProvider struct {
	port.IssueProvider
	update func(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
}

func (p auditProvider) UpdateIssueBodySection(ctx context.Context, req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return p.update(req)
}

type auditReceipt func(context.Context, string) (model.IssueOpsRecord, error)

func (r auditReceipt) Reflected(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	return r(ctx, id)
}

func TestAuditReflectorRequiresConfirmedRemoteUpdateBeforeReceipt(t *testing.T) {
	for _, tc := range []struct {
		name                     string
		providerMissing, updated bool
		remoteErr, receiptErr    error
		wantCalls                []string
	}{
		{name: "missing provider", providerMissing: true},
		{name: "remote failure", remoteErr: errors.New("remote failed"), wantCalls: []string{"remote"}},
		{name: "unconfirmed", wantCalls: []string{"remote"}},
		{name: "receipt failure", updated: true, receiptErr: errors.New("receipt failed"), wantCalls: []string{"remote", "receipt"}},
		{name: "confirmed", updated: true, wantCalls: []string{"remote", "receipt"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1"}
			snapshot := model.RemoteCompletionSection{CleanupAudit: "original snapshot"}
			ctx := context.WithValue(context.Background(), struct{}{}, "request")
			service := app.AuditReflector{Receipts: auditReceipt(func(got context.Context, id string) (model.IssueOpsRecord, error) {
				if got != ctx || id != record.ID {
					t.Fatal("receipt identity or context changed")
				}
				calls = append(calls, "receipt")
				return record, tc.receiptErr
			})}
			var provider port.IssueProvider
			if !tc.providerMissing {
				provider = auditProvider{update: func(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
					calls = append(calls, "remote")
					if req.Repo != record.Repo || req.IssueURL != record.IssueURL || req.Section != model.IssueBodySectionCompletion || !req.Confirm || req.Completion == nil || req.Completion.CleanupAudit != "audit" {
						t.Fatalf("request=%+v", req)
					}
					return port.IssueProviderUpdateIssueBodySectionResult{OK: true, Updated: tc.updated}, tc.remoteErr
				}}
			}
			err := service.Reflect(ctx, record, snapshot, "audit", provider)
			wantSuccess := tc.name == "confirmed"
			if (err == nil) != wantSuccess {
				t.Fatalf("err=%v", err)
			}
			if tc.remoteErr != nil && !errors.Is(err, tc.remoteErr) {
				t.Fatalf("remote error changed: %v", err)
			}
			if tc.receiptErr != nil && !errors.Is(err, tc.receiptErr) {
				t.Fatalf("receipt error changed: %v", err)
			}
			if !reflect.DeepEqual(calls, tc.wantCalls) {
				t.Fatalf("calls=%v", calls)
			}
			if snapshot.CleanupAudit != "original snapshot" {
				t.Fatal("completion snapshot mutated")
			}
		})
	}
}
