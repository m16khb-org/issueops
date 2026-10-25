package issueopspublication

import (
	"context"
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopspublication"
	domain "issueops/internal/domain/issueopspublication"
)

type ReconcileService struct {
	repository Repository
	provider   Provider
	verifier   Verifier
	settled    CrashSettlement
}

// CrashSettlement reports whether a pending create whose invocation is unknown
// can no longer land (see domain ReconcileFacts.CrashSettled).
type CrashSettlement func(ctx context.Context, intent contract.Intent) (bool, error)

// WithCrashSettlement lets reconcile retry a create whose caller died inside
// the provider call once the settlement proves nothing can still land.
func (s *ReconcileService) WithCrashSettlement(settled CrashSettlement) *ReconcileService {
	s.settled = settled
	return s
}

func NewReconcileService(repository Repository, provider Provider, verifier Verifier) *ReconcileService {
	return &ReconcileService{repository: repository, provider: provider, verifier: verifier}
}

func (s *ReconcileService) Reconcile(ctx context.Context, id string) (contract.ReconcileResult, error) {
	if s == nil || s.repository == nil || s.provider == nil || s.verifier == nil {
		return contract.ReconcileResult{}, fmt.Errorf("publication reconcile dependencies are required")
	}

	intent, err := s.repository.LoadIntent(ctx, id)
	if err != nil {
		return contract.ReconcileResult{}, err
	}
	inventory, attempted, err := s.provider.Inspect(ctx, intent)
	if err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, intent.KnownURL, err)
		cause := fmt.Errorf("remote reconcile transport is ambiguous; intent retained: %w", err)
		return s.failed(ctx, intent, "remote_reconcile_ambiguous", attempted, cause)
	}

	facts := domain.ReconcileFacts{
		CandidateCount:    len(inventory.Candidates),
		AuthoritativeZero: inventory.AuthoritativeZero,
		Invocation:        intent.InvocationState,
		RetryCount:        intent.RetryCount,
	}
	// A known URL with an empty authoritative list is contradictory evidence:
	// keep the intent rather than retry (security review 2026-10-25).
	if s.settled != nil && facts.AuthoritativeZero && facts.CandidateCount == 0 && facts.Invocation != contract.InvocationNotInvokedProven && strings.TrimSpace(intent.KnownURL) == "" {
		// An observation error leaves the intent preserved (fail closed).
		settled, settleErr := s.settled(ctx, intent)
		facts.CrashSettled = settled && settleErr == nil
	}
	decision, err := domain.DecideReconcile(facts)
	if err != nil {
		return s.failed(ctx, intent, "remote_reconcile_ambiguous", attempted, err)
	}

	switch decision.Action {
	case domain.ActionAdopt:
		return s.adopt(ctx, intent, inventory.Candidates[decision.CandidateIndex], attempted)
	case domain.ActionRetry:
		return s.retry(ctx, intent, attempted)
	case domain.ActionPreserve:
		return s.preserve(ctx, intent, decision.Reason, attempted)
	default:
		return s.failed(ctx, intent, "remote_reconcile_ambiguous", attempted, fmt.Errorf("unsupported publication reconcile action %q", decision.Action))
	}
}

func (s *ReconcileService) adopt(ctx context.Context, intent contract.Intent, candidate contract.Candidate, attempted bool) (contract.ReconcileResult, error) {
	if err := s.verifier.VerifyCandidate(ctx, intent, candidate); err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, intent.KnownURL, err)
		return s.failed(ctx, intent, "remote_reconcile_candidate_mismatch", attempted, err)
	}
	if err := s.verifier.VerifyLive(ctx, intent, candidate.URL); err != nil {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, candidate.URL, err)
		return s.failed(ctx, intent, "remote_reconcile_verification_failed", attempted, err)
	}
	record, err := s.repository.Complete(ctx, intent, candidate.URL, false)
	if err != nil {
		return s.failed(ctx, intent, "remote_reconcile_receipt_failed", attempted, err)
	}
	return reconciledResult(record, "remote_reconcile_adopted", attempted), nil
}

func (s *ReconcileService) retry(ctx context.Context, intent contract.Intent, attempted bool) (contract.ReconcileResult, error) {
	invoking, err := s.repository.MarkRetry(ctx, intent)
	if err != nil {
		return s.failed(ctx, intent, "remote_reconcile_retry_cas_failed", attempted, err)
	}
	created, invocation, createErr := s.provider.Create(ctx, invoking.Provider, invoking.Request)
	switch domain.DecideRetryOutcome(domain.RetryOutcomeFacts{Invocation: invocation, CallFailed: createErr != nil}) {
	case domain.RetryOutcomeTerminalNotInvoked:
		record, finishErr := s.repository.CompleteNotInvoked(ctx, invoking, createErr)
		if finishErr != nil {
			return s.failed(ctx, invoking, "remote_reconcile_retry_receipt_failed", attempted, finishErr)
		}
		return reconciledResult(record, "remote_reconcile_retry_not_invoked", attempted), createErr
	case domain.RetryOutcomePreserve:
		_ = s.repository.RecordFailure(ctx, invoking, contract.InvocationUnknown, created.URL, createErr)
		cause := fmt.Errorf("remote retry outcome is ambiguous; creation was not retried again: %w", createErr)
		return s.failed(ctx, invoking, "remote_reconcile_retry_ambiguous", attempted, cause)
	}
	if err := s.verifier.VerifyLive(ctx, invoking, created.URL); err != nil {
		_ = s.repository.RecordFailure(ctx, invoking, contract.InvocationUnknown, created.URL, err)
		return s.failed(ctx, invoking, "remote_reconcile_retry_verification_failed", attempted, err)
	}
	record, err := s.repository.Complete(ctx, invoking, created.URL, false)
	if err != nil {
		return s.failed(ctx, invoking, "remote_reconcile_retry_receipt_failed", attempted, err)
	}
	return reconciledResult(record, "remote_reconcile_retry_succeeded", attempted), nil
}

func (s *ReconcileService) preserve(ctx context.Context, intent contract.Intent, reason string, attempted bool) (contract.ReconcileResult, error) {
	var code string
	var cause error
	recordFailure := false

	switch reason {
	case "multiple-candidates":
		code = "remote_reconcile_multiple"
		cause = fmt.Errorf("remote reconcile found multiple candidates; intent retained")
		recordFailure = true
	case "non-authoritative-zero":
		code = "remote_reconcile_zero_ambiguous"
		cause = fmt.Errorf("remote reconcile returned a non-authoritative zero candidate result; intent retained")
		recordFailure = true
	case "unknown-invocation":
		code = "remote_reconcile_zero_unproven"
		// QA: agents read the conditions as requirements they could not meet;
		// the wording now says it is a wait (same as reconcile-issue).
		started := ""
		if intent.PendingStartedAt != "" {
			started = " (started " + intent.PendingStartedAt + ")"
		}
		cause = fmt.Errorf("not settled yet, wait and re-run this reconcile: authoritative zero cannot clear an invocation whose absence was not proven; intent retained until the create is 2 minutes old%s, no gh/glab create for its head is still running, and no PR URL is known", started)
	case "retry-exhausted":
		code = "remote_reconcile_retry_exhausted"
		cause = fmt.Errorf("remote create pre-invocation retry is unavailable or already consumed")
	default:
		code = "remote_reconcile_ambiguous"
		cause = fmt.Errorf("unsupported publication reconcile reason %q", reason)
	}
	if recordFailure {
		_ = s.repository.RecordFailure(ctx, intent, contract.InvocationUnknown, intent.KnownURL, cause)
	}
	return s.failed(ctx, intent, code, attempted, cause)
}

func (s *ReconcileService) failed(ctx context.Context, intent contract.Intent, code string, attempted bool, cause error) (contract.ReconcileResult, error) {
	record := intent.Record.Clone()
	if latest, err := s.repository.Latest(ctx, intent.Record.ID); err == nil {
		record = latest
	}
	return contract.ReconcileResult{Record: record, Code: code, ExternalStateInspected: attempted}, cause
}

func reconciledResult(record contract.RecordSnapshot, code string, attempted bool) contract.ReconcileResult {
	return contract.ReconcileResult{Record: record, Reconciled: true, Code: code, ExternalStateInspected: attempted}
}
