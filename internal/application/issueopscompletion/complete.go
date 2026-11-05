// Package issueopscompletion is the application layer for the issueops execution completion use case: it validates the
// caller's authority and the final-head and verification evidence, then asks a repository to move the
// cycle into the completed state. Persistence and git/filesystem checks sit behind its ports.
package issueopscompletion

import (
	"context"
	"fmt"

	completioncontract "issueops/internal/contract/issueopscompletion"
	completiondomain "issueops/internal/domain/issueopscompletion"
)

// Request carries one completion attempt: the target cycle ID, the lease generation the caller believes it holds,
// the caller identity with its process ancestry, and the evidence to record (final head,
// verification report path and notes, optional remote artifact URL). Confirm must be true
// for the completion to be accepted.
type Request struct {
	ID                     string
	Generation             uint64
	Actor                  completioncontract.Actor
	Ancestry               []completioncontract.ProcessReceipt
	CWD                    string
	FinalHead              string
	VerificationReportPath string
	Verification           []string
	RemoteArtifactURL      string
	Confirm                bool
}

// Result reports a completed Complete call: the cycle ID and the persisted execution state.
type Result struct {
	OK        bool
	ID        string
	Execution completioncontract.Execution
}

// Service completes a prepared cycle: it verifies the caller, validates the completion evidence against the
// recorded cycle state, and persists the transition through Repository.
type Service struct {
	repository  Repository
	environment Environment
	clock       clock
	verify      actorVerifier
}

// NewService builds a Service from its repository, environment, clock, and actor verifier.
// All four are required; Complete fails when any is nil.
func NewService(repository Repository, environment Environment, clock clock, verify actorVerifier) *Service {
	return &Service{repository: repository, environment: environment, clock: clock, verify: verify}
}

// Complete verifies the caller and the evidence, then atomically records the completion of request.ID.
// Retrying with identical evidence is idempotent; different evidence for an already completed
// execution is an error. The first failing validation is returned unchanged.
func (s *Service) Complete(ctx context.Context, request Request) (Result, error) {
	if !s.ready() {
		return Result{ID: request.ID}, fmt.Errorf("completion dependencies are required")
	}
	actor, err := s.verify(ctx, request.Actor, request.Ancestry)
	if err != nil {
		return Result{ID: request.ID}, err
	}
	verification, err := completiondomain.PrepareEvidence(request.Confirm, request.Verification, request.RemoteArtifactURL)
	if err != nil {
		return Result{ID: request.ID}, err
	}
	command := completioncontract.Command{
		Generation: request.Generation, Actor: actor, FinalHead: request.FinalHead,
		VerificationReportPath: request.VerificationReportPath, Verification: verification,
		RemoteArtifactURL: request.RemoteArtifactURL,
	}
	persisted, err := s.repository.Update(ctx, request.ID, func(before completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
		return s.completeRecord(ctx, request, command, before)
	})
	if err != nil {
		return Result{ID: request.ID}, err
	}
	return Result{OK: true, ID: request.ID, Execution: persisted.Execution}, nil
}

func (s *Service) ready() bool {
	return s != nil && s.repository != nil && s.environment != nil && s.clock != nil && s.verify != nil
}

// retryIsIdempotent reports whether an existing completion matches the retried command evidence.
func (s *Service) retryIsIdempotent(before completioncontract.RecordSnapshot, command completioncontract.Command, remoteArtifactURL string) bool {
	if !completiondomain.CanRetryCompletion(toDomainSnapshot(before), command) {
		return false
	}
	pathsMatch := s.environment.PathsMatch(before.Completion.VerificationReportPath, command.VerificationReportPath)
	return completiondomain.MatchesRetryEvidence(*before.Completion, command, pathsMatch) && completiondomain.ValidateArtifact(before, remoteArtifactURL) == nil
}

func (s *Service) completeRecord(ctx context.Context, request Request, command completioncontract.Command, before completioncontract.RecordSnapshot) (completioncontract.RecordSnapshot, bool, error) {
	if err := completiondomain.ValidatePrepared(before.Prepared); err != nil {
		return before, false, err
	}
	if before.Completion != nil {
		if s.retryIsIdempotent(before, command, request.RemoteArtifactURL) {
			return before, false, nil
		}
		return before, false, fmt.Errorf("execution completion already exists with different evidence")
	}
	if err := completiondomain.ValidatePhase(before.Phase); err != nil {
		return before, false, err
	}
	if err := completiondomain.ValidateArtifact(before, request.RemoteArtifactURL); err != nil {
		return before, false, err
	}
	canonicalCWD := s.environment.PathsMatch(request.CWD, before.CanonicalRoot)
	if err := completiondomain.ValidateActive(toDomainSnapshot(before), command, canonicalCWD); err != nil {
		return before, false, publicDomainError(err, request.Generation)
	}
	head, err := s.environment.CurrentHead(ctx, before.CanonicalRoot)
	if err != nil {
		return before, false, err
	}
	if err := completiondomain.ValidateFinalHead(request.FinalHead, head); err != nil {
		return before, false, err
	}
	report, err := s.environment.VerifyReport(before.CanonicalRoot, request.VerificationReportPath)
	if err != nil {
		return before, false, err
	}
	if err := s.verifyChangesSincePR(ctx, before, head, report); err != nil {
		return before, false, err
	}
	completedAt := s.clock.Now()
	transitionedAt := s.clock.Now()
	outcome := completiondomain.ApplyAt(toDomainSnapshot(before), command, report, completedAt, transitionedAt)
	return fromDomainOutcome(before, outcome), true, nil
}

// verifyChangesSincePR requires that nothing verified-relevant changed since the PR head was recorded.
func (s *Service) verifyChangesSincePR(ctx context.Context, before completioncontract.RecordSnapshot, head, report string) error {
	if before.PRHead == "" {
		return nil
	}
	changed, err := s.environment.ChangedPathsSince(ctx, before.CanonicalRoot, before.PRHead, head, report)
	if err != nil {
		return err
	}
	return completiondomain.ValidateVerifiedSincePR(before.PRHead, changed)
}

func toDomainSnapshot(record completioncontract.RecordSnapshot) completiondomain.Snapshot {
	return completiondomain.Snapshot{Phase: record.Phase, Lease: record.Lease, Completion: record.Completion, Ledger: record.Ledger}
}

func fromDomainOutcome(before completioncontract.RecordSnapshot, outcome completiondomain.Outcome) completioncontract.RecordSnapshot {
	result := before.Clone()
	result.Phase = outcome.Phase
	result.Lease = outcome.Lease
	result.Completion = outcome.Completion
	result.Ledger = outcome.Ledger
	return result
}

func publicDomainError(err error, generation uint64) error {
	switch completiondomain.CodeOf(err) {
	case completiondomain.DenyAuthority:
		return fmt.Errorf("only the current holder may complete generation %d", generation)
	case completiondomain.DenyCWD:
		return fmt.Errorf("completion cwd must be the canonical worktree")
	default:
		return err
	}
}
