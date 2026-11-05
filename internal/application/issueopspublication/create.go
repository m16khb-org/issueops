// Package issueopspublication is the application layer for creating remote issue and
// pull-request publications safely: CreateService journals an intent before calling the
// provider once, and ReconcileService later resolves intents whose outcome is unknown by
// adopting, retrying, or preserving them. Providers, persistence, and verification sit behind ports.
package issueopspublication

import (
	"context"
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopspublication"
	domain "issueops/internal/domain/issueopspublication"
)

// CreateService creates one remote publication per confirmed command, journaling the intent
// first so a crash or ambiguous provider failure is resolved by ReconcileService instead of a blind retry.
type CreateService struct {
	preparer   preparer
	repository Repository
	provider   Provider
	verifier   verifier
}

// NewCreateService wires a CreateService; Create fails if any dependency is nil.
func NewCreateService(preparer preparer, repository Repository, provider Provider, verifier verifier) *CreateService {
	return &CreateService{preparer: preparer, repository: repository, provider: provider, verifier: verifier}
}

// Create prepares command and, without Confirm, only validates eligibility and returns the provider's unconfirmed result without journaling.
// With Confirm it journals the intent, invokes the provider once, and verifies and records the receipt;
// a failed or ambiguous remote create is never retried here and must go through execution reconcile.
func (s *CreateService) Create(ctx context.Context, command contract.CreateCommand) (contract.ProviderCreateResult, error) {
	if s == nil || s.preparer == nil || s.repository == nil || s.provider == nil || s.verifier == nil {
		return contract.ProviderCreateResult{}, fmt.Errorf("publication create dependencies are required")
	}
	prepared, err := s.preparer.Prepare(ctx, command)
	if err != nil {
		return contract.ProviderCreateResult{}, err
	}
	if !command.Confirm {
		if _, err := domain.ValidateCreateEligibility(prepared.Eligibility); err != nil {
			return contract.ProviderCreateResult{}, err
		}
		result, _, err := s.provider.Create(ctx, prepared.Eligibility.Provider, prepared.Request)
		return result, err
	}
	return s.createConfirmed(ctx, prepared)
}

// createConfirmed journals the intent, invokes the provider once, and records
// the receipt or the failure that execution reconcile must resolve.
func (s *CreateService) createConfirmed(ctx context.Context, prepared contract.PreparedCreate) (contract.ProviderCreateResult, error) {
	intent, err := s.repository.BeginCreate(ctx, prepared)
	if err != nil {
		return contract.ProviderCreateResult{}, err
	}
	if _, err := domain.ValidateCreateEligibility(intent.Eligibility); err != nil {
		return contract.ProviderCreateResult{}, err
	}
	result, invocation, callErr := s.provider.Create(ctx, intent.Provider, intent.Request)
	if callErr != nil {
		_ = s.repository.RecordFailure(ctx, intent, invocation, result.URL, callErr)
		return result, fmt.Errorf("remote create outcome requires execution reconcile; creation was not retried: %w", callErr)
	}
	if strings.TrimSpace(result.URL) == "" {
		cause := fmt.Errorf("provider create returned no canonical URL")
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, "", cause)
		return result, cause
	}
	if err := s.verifier.VerifyLive(ctx, intent, result.URL); err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, result.URL, err)
		return result, fmt.Errorf("provider returned a URL but durable verification requires execution reconcile: %w", err)
	}
	if _, err := s.repository.Complete(ctx, intent, result.URL, true); err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, result.URL, err)
		return result, fmt.Errorf("provider succeeded but durable receipt requires execution reconcile: %w", err)
	}
	return result, nil
}
