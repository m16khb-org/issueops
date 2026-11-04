// Package issueopscleanup implements the application layer of IssueOps cycle
// cleanup: previewing and applying finish, abandon, remote-branch,
// linked-branch and orphan cleanup, closing child tasks, and reporting cleanup
// status. It orchestrates domain rules and ports under fingerprint, lifetime
// and record-lock fences, and owns no I/O of its own.
package issueopscleanup

import (
	"context"
	"errors"
	"fmt"

	provenance "issueops/internal/application/issueopsprovenance"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

// Invocation connects record/provider observations and next-command provenance
// to the cleanup executors while preserving their observation and mutation fences.
type Invocation struct {
	Read                       func(string) (model.Record, error)
	Provider                   func(string) (port.IssueProvider, error)
	CurrentDirectory           func() (string, error)
	Provenance                 provenanceport.Observer
	MergeVerificationAvailable bool
	RunFinish                  func(context.Context, model.CleanupFinishRequest, port.IssueProvider) (model.CleanupFinishResult, error)
	RunRemoteBranch            func(context.Context, model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchResult, error)
	RunAbandon                 func(context.Context, model.CleanupAbandonRequest) (model.CleanupAbandonResult, error)
	RunLinkedBranch            func(context.Context, model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error)
}

// Finish runs a cleanup finish for the record, resolving the provider (the
// override, else the record's) and the current directory, and binds the next
// command to provenance. Failures before the run are CleanupInvocationError.
func (s Invocation) Finish(ctx context.Context, req model.CleanupFinishRequest, providerOverride string) (model.CleanupFinishResult, error) {
	var empty model.CleanupFinishResult
	record, err := s.Read(req.ID)
	if err != nil {
		return empty, &port.CleanupInvocationError{Err: err}
	}
	name := providerOverride
	if name == "" {
		name = domain.ResolveRecordProvider(record)
	}
	if name == "" {
		return empty, &port.CleanupInvocationError{Err: fmt.Errorf("cannot determine provider from IssueOps record; pass --provider")}
	}
	provider, err := s.Provider(name)
	if err != nil {
		return empty, &port.CleanupInvocationError{Err: err}
	}
	cwd, err := s.CurrentDirectory()
	if err != nil {
		return empty, &port.CleanupInvocationError{Err: fmt.Errorf("cannot resolve current directory (refusing destructive cleanup): %w", err)}
	}
	req.ID, req.CWD = record.ID, cwd
	result, err := s.RunFinish(ctx, req, provider)
	if _, ok := errors.AsType[*port.CleanupFinishObservationError](err); ok {
		return result, &port.CleanupInvocationError{Err: err}
	}
	bound, bindErr := BindNextCommand(ctx, result.NextCommand, domain.CleanupCommandGeneration(record), s.Provenance)
	if bindErr != nil {
		return result, &port.CleanupInvocationError{Err: bindErr}
	}
	result.NextCommand = bound
	return result, err
}

// RemoteBranch runs a remote-branch cleanup after checking the record's
// provider and merge verification are available, and binds the next command to
// provenance.
func (s Invocation) RemoteBranch(ctx context.Context, req model.CleanupRemoteBranchRequest) (model.CleanupRemoteBranchResult, error) {
	var empty model.CleanupRemoteBranchResult
	record, err := s.Read(req.ID)
	if err != nil {
		return empty, &port.CleanupInvocationError{Err: err}
	}
	name := domain.ResolveRecordProvider(record)
	if name == "" {
		return empty, &port.CleanupInvocationError{Err: fmt.Errorf("cannot determine provider from IssueOps record")}
	}
	if _, err := s.Provider(name); err != nil {
		return empty, &port.CleanupInvocationError{Err: err}
	}
	if !s.MergeVerificationAvailable {
		return empty, &port.CleanupInvocationError{Err: fmt.Errorf("merge verification is not configured")}
	}
	result, err := s.RunRemoteBranch(ctx, req)
	bound, bindErr := BindNextCommand(ctx, result.NextCommand, domain.CleanupCommandGeneration(record), s.Provenance)
	if bindErr != nil {
		return result, &port.CleanupInvocationError{Err: bindErr}
	}
	result.NextCommand = bound
	return result, err
}

// Abandon runs a cleanup abandon and binds any next command it returns to
// provenance, reading the record only when there is a command to bind.
func (s Invocation) Abandon(ctx context.Context, req model.CleanupAbandonRequest) (model.CleanupAbandonResult, error) {
	result, err := s.RunAbandon(ctx, req)
	if result.NextCommand != "" {
		record, readErr := s.Read(req.ID)
		if readErr != nil {
			return result, &port.CleanupInvocationError{Err: readErr}
		}
		bound, bindErr := BindNextCommand(ctx, result.NextCommand, domain.CleanupCommandGeneration(record), s.Provenance)
		if bindErr != nil {
			return result, &port.CleanupInvocationError{Err: bindErr}
		}
		result.NextCommand = bound
	}
	return result, err
}

// LinkedBranch runs a linked-branch cleanup for the record and binds the next
// command to provenance.
func (s Invocation) LinkedBranch(ctx context.Context, req model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error) {
	record, err := s.Read(req.ID)
	if err != nil {
		return model.CleanupLinkedBranchResult{}, &port.CleanupInvocationError{Err: err}
	}
	result, err := s.RunLinkedBranch(ctx, req)
	bound, bindErr := BindNextCommand(ctx, result.NextCommand, domain.CleanupCommandGeneration(record), s.Provenance)
	if bindErr != nil {
		return result, &port.CleanupInvocationError{Err: bindErr}
	}
	result.NextCommand = bound
	return result, err
}

// BindNextCommand binds command to the record generation through observer when
// the command needs provenance; otherwise it returns command unchanged.
func BindNextCommand(ctx context.Context, command string, generation uint64, observer provenanceport.Observer) (string, error) {
	if !domain.CleanupCommandNeedsProvenance(command, generation) {
		return command, nil
	}
	return provenance.Bind(ctx, command, generation, observer)
}
