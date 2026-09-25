package nativeactivation

import (
	"context"
	"fmt"

	activationcontract "issueops/internal/contract/nativeactivation"
	activationdomain "issueops/internal/domain/nativeactivation"
	activationport "issueops/internal/port/nativeactivation"
)

type Service struct {
	backend  activationport.Backend
	readback activationport.ReadbackVerifier
}

func NewService(backend activationport.Backend, readback activationport.ReadbackVerifier) *Service {
	return &Service{backend: backend, readback: readback}
}

func (service *Service) Begin(ctx context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	if service == nil || service.backend == nil {
		return activationcontract.Result{}, fmt.Errorf("native activation backend is required")
	}
	if err := activationdomain.ValidateRequest(request); err != nil {
		return activationcontract.Result{}, err
	}
	if request.TransitionID != "" {
		return activationcontract.Result{}, fmt.Errorf("native activation begin must not provide a transition ID")
	}
	result, err := service.backend.Begin(ctx, activationport.BeginRequest{StateRoot: request.StateRoot, IssueOpsRoot: request.IssueOpsRoot, TargetBinary: request.TargetBinary})
	if err != nil {
		return activationcontract.Result{}, err
	}
	if err := validateBackendResult(request, result, false); err != nil {
		return activationcontract.Result{}, err
	}
	return publicResult(result, activationport.Readback{}), nil
}

func (service *Service) Seal(ctx context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	if service == nil || service.backend == nil || service.readback == nil {
		return activationcontract.Result{}, fmt.Errorf("native activation dependencies are required")
	}
	if err := activationdomain.ValidateRequest(request); err != nil {
		return activationcontract.Result{}, err
	}
	if !activationdomain.ValidTransitionID(request.TransitionID) {
		return activationcontract.Result{}, fmt.Errorf("native activation seal requires the exact transition ID")
	}
	readback, err := service.readback.Verify(ctx, request.IssueOpsRoot, request.TargetBinary)
	if err != nil {
		return activationcontract.Result{}, err
	}
	readback, err = validateReadback(readback)
	if err != nil {
		return activationcontract.Result{}, err
	}
	result, err := service.backend.Seal(ctx, activationport.SealRequest{
		StateRoot: request.StateRoot, IssueOpsRoot: request.IssueOpsRoot, TargetBinary: request.TargetBinary,
		TransitionID:  request.TransitionID,
		CatalogSHA256: readback.CatalogSHA256,
		Evidence:      append([]activationport.Evidence(nil), readback.Evidence...),
	})
	if err != nil {
		return activationcontract.Result{}, err
	}
	if err := validateBackendResult(request, result, true); err != nil {
		return activationcontract.Result{}, err
	}
	return publicResult(result, readback), nil
}

func (service *Service) Abort(ctx context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	if service == nil || service.backend == nil {
		return activationcontract.Result{}, fmt.Errorf("native activation backend is required")
	}
	if err := activationdomain.ValidateRequest(request); err != nil {
		return activationcontract.Result{}, err
	}
	if !activationdomain.ValidTransitionID(request.TransitionID) {
		return activationcontract.Result{}, fmt.Errorf("native activation abort requires the exact transition ID")
	}
	result, err := service.backend.Abort(ctx, activationport.AbortRequest{
		StateRoot: request.StateRoot, IssueOpsRoot: request.IssueOpsRoot, TargetBinary: request.TargetBinary, TransitionID: request.TransitionID,
	})
	if err != nil {
		return activationcontract.Result{}, err
	}
	if err := validateBackendIdentity(request, result); err != nil || !result.Aborted || result.Pending || result.Sealed || !activationdomain.ValidSHA256(result.BinarySHA256) {
		return activationcontract.Result{}, fmt.Errorf("native activation backend did not abort the pending transition")
	}
	return publicResult(result, activationport.Readback{}), nil
}

func validateBackendResult(request activationcontract.Request, result activationport.Result, sealed bool) error {
	if err := validateBackendIdentity(request, result); err != nil {
		return err
	}
	if sealed {
		if !result.Sealed || result.Pending || !activationdomain.ValidSHA256(result.BinarySHA256) {
			return fmt.Errorf("native activation backend did not seal the receipt")
		}
		return nil
	}
	if !result.Pending || result.Sealed || !activationdomain.ValidSHA256(result.BinarySHA256) {
		return fmt.Errorf("native activation backend did not persist a pending activation")
	}
	return nil
}

func validateBackendIdentity(request activationcontract.Request, result activationport.Result) error {
	if result.StateRoot != request.StateRoot || result.IssueOpsRoot != request.IssueOpsRoot || result.TargetBinary != request.TargetBinary ||
		(request.TransitionID != "" && result.TransitionID != request.TransitionID) || !activationdomain.ValidTransitionID(result.TransitionID) {
		return fmt.Errorf("native activation backend identity mismatch")
	}
	if !activationdomain.ValidTimestamp(result.UpdatedAt) {
		return fmt.Errorf("native activation backend transition timestamp is invalid")
	}
	return nil
}

func validateReadback(readback activationport.Readback) (activationport.Readback, error) {
	facts := make([]activationcontract.Evidence, len(readback.Evidence))
	for i, evidence := range readback.Evidence {
		facts[i] = activationcontract.Evidence{
			Host: evidence.Host, Surface: evidence.Surface, Path: evidence.Path,
			SemanticSHA256: evidence.SemanticSHA256, SHA256: evidence.SHA256,
		}
	}
	order, err := activationdomain.ReadbackOrder(readback.CatalogSHA256, facts)
	if err != nil {
		return activationport.Readback{}, err
	}
	sorted := make([]activationport.Evidence, len(order))
	for i, original := range order {
		sorted[i] = readback.Evidence[original]
	}
	readback.Evidence = sorted
	return readback, nil
}

func publicResult(result activationport.Result, readback activationport.Readback) activationcontract.Result {
	evidence := make([]activationcontract.Evidence, 0, len(readback.Evidence))
	for _, item := range readback.Evidence {
		evidence = append(evidence, activationcontract.Evidence{
			Host: item.Host, Surface: item.Surface, Path: item.Path, SemanticSHA256: item.SemanticSHA256,
			SHA256: item.SHA256, Mode: item.Mode, Size: item.Size, Device: item.Device, Inode: item.Inode,
		})
	}
	public := activationcontract.Result{
		OK: true, StateRoot: result.StateRoot, IssueOpsRoot: result.IssueOpsRoot, TargetBinary: result.TargetBinary,
		BinarySHA256: result.BinarySHA256, TransitionID: result.TransitionID, Pending: result.Pending, Sealed: result.Sealed, Aborted: result.Aborted, UpdatedAt: result.UpdatedAt,
	}
	if result.Sealed {
		public.Receipt = &activationcontract.Receipt{
			SchemaVersion: activationcontract.SchemaVersion, StateRoot: result.StateRoot, IssueOpsRoot: result.IssueOpsRoot,
			TargetBinary: result.TargetBinary, BinarySHA256: result.BinarySHA256, TransitionID: result.TransitionID, CatalogSHA256: readback.CatalogSHA256,
			Evidence: evidence, SealedAt: result.UpdatedAt,
		}
	}
	return public
}
