package issueopscleanup

import (
	"context"
	"fmt"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type CompletionReflectionReceipts interface {
	Reflected(context.Context, string) (model.IssueOpsRecord, error)
}

type AuditReflector struct{ Receipts CompletionReflectionReceipts }

// Reflect uses the completion snapshot captured before workspace deletion.
// Only a confirmed provider update may advance the local reflection receipt.
func (s AuditReflector) Reflect(ctx context.Context, record model.IssueOpsRecord, completion model.RemoteCompletionSection, audit string, prov port.IssueProvider) error {
	if prov == nil {
		return fmt.Errorf("no issue provider configured")
	}
	completion.CleanupAudit = audit
	result, err := prov.UpdateIssueBodySection(ctx, port.IssueProviderUpdateIssueBodySectionRequest{
		Repo: record.Repo, IssueURL: record.IssueURL, Section: model.IssueBodySectionCompletion, Completion: &completion, Confirm: true,
	})
	if err != nil {
		return err
	}
	if !result.Updated {
		return fmt.Errorf("cleanup audit reflection was not confirmed")
	}
	_, err = s.Receipts.Reflected(ctx, record.ID)
	return err
}
