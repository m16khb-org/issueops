package issueops

import (
	"context"
	"fmt"

	application "issueops/internal/application/issueopsbodysync"
	"issueops/internal/port"
)

// BodySyncProvider narrows an issue provider to the capabilities that artifact body sync
// needs: reading and replacing bodies, and optionally verifying child hierarchy.
type BodySyncProvider struct {
	port.IssueProviderArtifactBodyReader
	port.IssueProviderArtifactBodyReplacer
	provider  port.IssueProvider
	hierarchy port.IssueProviderChildHierarchyVerifier
}

// NewBodySyncProvider wraps provider, failing when it is nil or cannot both read and replace
// artifact bodies. Child hierarchy verification is optional.
func NewBodySyncProvider(provider port.IssueProvider) (BodySyncProvider, error) {
	if provider == nil {
		return BodySyncProvider{}, fmt.Errorf("no issue provider configured")
	}
	reader, ok := provider.(port.IssueProviderArtifactBodyReader)
	if !ok {
		return BodySyncProvider{}, fmt.Errorf("provider %q cannot read artifact bodies", provider.Name())
	}
	replacer, ok := provider.(port.IssueProviderArtifactBodyReplacer)
	if !ok {
		return BodySyncProvider{}, fmt.Errorf("provider %q cannot replace artifact bodies", provider.Name())
	}
	hierarchy, _ := provider.(port.IssueProviderChildHierarchyVerifier)
	return BodySyncProvider{IssueProviderArtifactBodyReader: reader, IssueProviderArtifactBodyReplacer: replacer, provider: provider, hierarchy: hierarchy}, nil
}

// Name returns the wrapped provider name.
func (p BodySyncProvider) Name() string { return p.provider.Name() }

// VerifyChildHierarchy delegates to the provider, or fails when the provider cannot verify
// child hierarchy.
func (p BodySyncProvider) VerifyChildHierarchy(ctx context.Context, req port.IssueProviderChildHierarchyRequest) (port.IssueProviderChildHierarchyResult, error) {
	if p.hierarchy == nil {
		return port.IssueProviderChildHierarchyResult{}, fmt.Errorf("provider %q cannot verify child hierarchy, so a child body cannot be synced", p.Name())
	}
	return p.hierarchy.VerifyChildHierarchy(ctx, req)
}

var _ application.Provider = BodySyncProvider{}
