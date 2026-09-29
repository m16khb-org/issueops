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
	Read                       func(string) (model.IssueOpsRecord, error)
	Provider                   func(string) (port.IssueProvider, error)
	CurrentDirectory           func() (string, error)
	Provenance                 provenanceport.Observer
	MergeVerificationAvailable bool
	RunFinish                  func(context.Context, model.CleanupFinishRequest, port.IssueProvider) (model.CleanupFinishResult, error)
	RunRemoteBranch            func(context.Context, model.CleanupRemoteBranchRequest, port.IssueProvider) (model.CleanupRemoteBranchResult, error)
	RunAbandon                 func(context.Context, model.CleanupAbandonRequest) (model.CleanupAbandonResult, error)
	RunLinkedBranch            func(context.Context, model.CleanupLinkedBranchRequest) (model.CleanupLinkedBranchResult, error)
}

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
	provider, err := s.Provider(name)
	if err != nil {
		return empty, &port.CleanupInvocationError{Err: err}
	}
	if !s.MergeVerificationAvailable {
		return empty, &port.CleanupInvocationError{Err: fmt.Errorf("merge verification is not configured")}
	}
	result, err := s.RunRemoteBranch(ctx, req, provider)
	bound, bindErr := BindNextCommand(ctx, result.NextCommand, domain.CleanupCommandGeneration(record), s.Provenance)
	if bindErr != nil {
		return result, &port.CleanupInvocationError{Err: bindErr}
	}
	result.NextCommand = bound
	return result, err
}
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
func BindNextCommand(ctx context.Context, command string, generation uint64, observer provenanceport.Observer) (string, error) {
	if !domain.CleanupCommandNeedsProvenance(command, generation) {
		return command, nil
	}
	return provenance.Bind(ctx, command, generation, observer)
}
