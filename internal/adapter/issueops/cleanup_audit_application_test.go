package issueops

import (
	"context"
	"testing"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	completionapp "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type cleanupAuditUpdatingProvider struct {
	port.IssueProvider
	update func(port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error)
}

func (p cleanupAuditUpdatingProvider) UpdateIssueBodySection(ctx context.Context, req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
	return p.update(req)
}

func TestCleanupAuditUsesPreDeletionSnapshotAndPreservesLatestRecord(t *testing.T) {
	root, record := completionTestRecord(t)
	snapshot := model.RemoteCompletionSection{PlanBody: "sealed plan before workspace removal", FinalHead: "sealed-head"}
	now := time.Date(2026, time.September, 20, 0, 0, 0, 0, time.UTC)
	provider := cleanupAuditUpdatingProvider{update: func(req port.IssueProviderUpdateIssueBodySectionRequest) (port.IssueProviderUpdateIssueBodySectionResult, error) {
		if req.Completion == nil || req.Completion.PlanBody != snapshot.PlanBody || req.Completion.FinalHead != snapshot.FinalHead || req.Completion.CleanupAudit != "cleanup applied" {
			t.Fatalf("preserved payload lost: %+v", req)
		}
		err := withIssueOpsLock(context.Background(), root, record.ID, func(context.Context) error {
			latest, err := ReadIssueOps(root, record.ID)
			if err != nil {
				return err
			}
			latest.PlanPath = "concurrent-plan.md"
			_, err = writeIssueOps(root, latest)
			return err
		})
		return port.IssueProviderUpdateIssueBodySectionResult{Updated: true}, err
	}}
	service := cleanupapp.AuditReflector{Receipts: completionapp.NewCompletionReceipts(RemoteRecordStore{StateRoot: root}, func() time.Time { return now })}
	if err := service.Reflect(context.Background(), record, snapshot, "cleanup applied", provider); err != nil {
		t.Fatal(err)
	}
	latest, err := ReadIssueOps(root, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.PlanPath != "concurrent-plan.md" || latest.RemoteCompletion == nil || latest.RemoteCompletion.ReflectedAt != now.Format(time.RFC3339Nano) {
		t.Fatalf("latest record overwritten or receipt absent: %+v", latest)
	}
	if snapshot.CleanupAudit != "" {
		t.Fatal("input completion snapshot changed")
	}
}
