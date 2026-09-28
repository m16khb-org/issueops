package issueopscompletion

import (
	"context"
	"fmt"

	completioncontract "issueops/internal/contract/issueopscompletion"
	completiondomain "issueops/internal/domain/issueopscompletion"
)

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

type Result struct {
	OK        bool
	ID        string
	Execution completioncontract.Execution
}

type Service struct {
	repository  Repository
	environment Environment
	clock       Clock
	inspect     ProcessInspector
}

func NewService(repository Repository, environment Environment, clock Clock, inspect ProcessInspector) *Service {
	return &Service{repository: repository, environment: environment, clock: clock, inspect: inspect}
}

func (s *Service) Complete(ctx context.Context, request Request) (Result, error) {
	if s == nil || s.repository == nil || s.environment == nil || s.clock == nil || s.inspect == nil {
		return Result{ID: request.ID}, fmt.Errorf("completion dependencies are required")
	}
	actor, err := resolveActor(ctx, request.Actor, request.Ancestry, s.inspect)
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
		if err := completiondomain.ValidatePrepared(before.Prepared); err != nil {
			return before, false, err
		}
		if before.Completion != nil {
			if completiondomain.CanRetryCompletion(toDomainSnapshot(before), command) {
				pathsMatch := s.environment.PathsMatch(before.Completion.VerificationReportPath, command.VerificationReportPath)
				if completiondomain.MatchesRetryEvidence(*before.Completion, command, pathsMatch) && completiondomain.ValidateArtifact(before, request.RemoteArtifactURL) == nil {
					return before, false, nil
				}
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
		completedAt := s.clock.Now()
		transitionedAt := s.clock.Now()
		outcome := completiondomain.ApplyAt(toDomainSnapshot(before), command, report, completedAt, transitionedAt)
		return fromDomainOutcome(before, outcome), true, nil
	})
	if err != nil {
		return Result{ID: request.ID}, err
	}
	return Result{OK: true, ID: request.ID, Execution: persisted.Execution}, nil
}

func resolveActor(ctx context.Context, actor completioncontract.Actor, ancestry []completioncontract.ProcessReceipt, inspect ProcessInspector) (completioncontract.Actor, error) {
	actor, err := completiondomain.NormalizeActor(actor, ancestry)
	if err != nil {
		return completioncontract.Actor{}, err
	}
	status, observed, err := inspect(ctx, *actor.Process)
	if err != nil {
		return completioncontract.Actor{}, err
	}
	if err := completiondomain.ValidateLiveActor(actor, status, observed); err != nil {
		return completioncontract.Actor{}, err
	}
	return actor, nil
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
