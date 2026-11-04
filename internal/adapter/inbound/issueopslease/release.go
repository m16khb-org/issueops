// Package issueopslease maps execution requests to the release use case.
package issueopslease

import (
	"context"
	"errors"
	"fmt"

	leaseapp "issueops/internal/application/issueopslease"
	issueopscontract "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type releaseHandler struct{ service *leaseapp.ReleaseService }

// NewReleaseHandler adapts service to an execution release handler. The
// handler fails with ErrReleaseHandlerUnavailable when service is nil.
func NewReleaseHandler(service *leaseapp.ReleaseService) issueopscontract.ExecutionReleaseHandler {
	handler := releaseHandler{service: service}
	return handler.Handle
}

func (h releaseHandler) Handle(ctx context.Context, _ string, request issueopscontract.ExecutionReleaseRequest) (issueopscontract.ExecutionResult, error) {
	if h.service == nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, issueopscontract.ErrReleaseHandlerUnavailable
	}
	result, err := h.service.Release(ctx, leaseapp.ReleaseRequest{ID: request.ID, Generation: request.Generation, Actor: toDomainActor(request.Actor), Ancestry: toProcessAncestry(request.Actor), CWD: request.CWD})
	if err != nil {
		return issueopscontract.ExecutionResult{ID: request.ID}, publicReleaseError(err, request.ID, request.Generation)
	}
	return issueopscontract.ExecutionResult{OK: result.OK, ID: result.ID, Execution: result.Execution.ToIssueOps()}, nil
}

func publicReleaseError(err error, id string, generation uint64) error {
	if code := leasedomain.DenyCodeOf(err); code == leasedomain.DenyLeaseAuthority {
		// Probe: the holder itself passed a stale --generation (9 for 2) and
		// read that someone else held the lease.
		return fmt.Errorf("only the current holder may release generation %d: release needs an active lease held by this actor at exactly that generation; issueops execution status --id %s --json shows lease.status, lease.generation and lease.holder", generation, id)
	} else if code == leasedomain.DenyCanonicalCWD {
		return canonicalCWDError("release", err)
	}
	failure, ok := errors.AsType[*leasecontract.Failure](err)
	if !ok {
		return err
	}
	return failure.Cause
}

func toDomainActor(actor issueopscontract.NativeActor) leasedomain.Actor {
	result := leasedomain.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.Process = &leasedomain.ProcessReceipt{PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable}
	}
	return result
}

func toProcessAncestry(actor issueopscontract.NativeActor) []leasedomain.ProcessReceipt {
	ancestry := make([]leasedomain.ProcessReceipt, 0, len(actor.ProcessAncestry))
	for _, receipt := range actor.ProcessAncestry {
		ancestry = append(ancestry, leasedomain.ProcessReceipt{PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable})
	}
	return ancestry
}
