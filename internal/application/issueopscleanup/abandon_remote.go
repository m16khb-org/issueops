package issueopscleanup

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

type AbandonRemoteObserver struct {
	RemoteRef func(context.Context, string, string) (string, error)
}

func (s AbandonRemoteObserver) Observe(ctx context.Context, record model.IssueOpsRecord, req model.CleanupAbandonRequest, provider port.IssueProvider, inventory model.CleanupAbandonInventory) (model.CleanupAbandonInventory, domain.CleanupAbandonObservation) {
	plan := domain.PlanCleanupAbandonRemoteReads(record, req, inventory.Branch, provider != nil)
	var facts domain.CleanupAbandonRemoteFacts
	read := func(kind, url string) (string, error) {
		reader, ok := provider.(port.IssueProviderArtifactBodyReader)
		if !ok {
			return "", fmt.Errorf("provider does not support reading artifact state")
		}
		kind = strings.TrimSpace(kind)
		if kind == "" {
			kind = "issue"
		}
		body, err := reader.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{Repo: record.Repo, Kind: kind, URL: strings.TrimSpace(url)})
		return body.State, err
	}
	if plan.Artifact {
		facts.ArtifactState, facts.ArtifactError = read(record.RemoteArtifact.Kind, record.RemoteArtifact.URL)
	}
	if plan.Issue {
		facts.IssueState, facts.IssueError = read("issue", record.IssueURL)
	}
	if plan.Branch {
		facts.BranchOID, facts.BranchError = s.RemoteRef(ctx, record.Repo, inventory.Branch)
	}
	return domain.BuildCleanupAbandonRemotePreview(req, inventory, plan, facts)
}
