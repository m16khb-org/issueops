package issueopscmux

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	auditcontract "issueops/internal/contract/audit"
	cmuxcontract "issueops/internal/contract/cmux"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	port "issueops/internal/port/cmux"
)

type Service struct {
	Client                 port.Client
	Roots                  port.RootEnvironment
	Now                    func() time.Time
	ReadAudit              func() ([]issueopscontract.IssueOpsHandoffDeliveryObservation, error)
	Observe                func(issueopscontract.IssueOpsRecord, issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)
	ArtifactRoot           string
	SamePath               func(string, string) bool
	ReadPrompt             func(string, string, string) ([]byte, error)
	ValidateHostExecutable func(string) error
	ValidateHostProfile    func(string, string, string, string) error
	Prepare                func(cmuxcontract.ArtifactRequest) (cmuxcontract.PreparedLauncher, error)
	AwaitReceipt           func(context.Context, cmuxcontract.PreparedLauncher, cmuxcontract.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error)
	Cleanup                func(cmuxcontract.PreparedLauncher) error
}

func (deps Service) Handoff(ctx context.Context, stateRoot string, request issueopscontract.ExecutionCmuxHandoffRequest) (issueopscontract.ExecutionCmuxHandoffResult, error) {
	result := issueopscontract.ExecutionCmuxHandoffResult{ID: request.ID, Generation: request.Generation, Status: "rejected"}
	if deps.Client == nil || deps.Now == nil || deps.Roots.GitTop == nil || deps.ValidateHostExecutable == nil {
		return result, fmt.Errorf("cmux handoff dependencies are unavailable")
	}
	if issueopsdomain.CmuxRequestContainsNUL(request) {
		return result, fmt.Errorf("cmux handoff identity must not contain NUL bytes")
	}
	fence, record, err := openRootFence(deps.Roots, stateRoot, request)
	if err != nil {
		return result, err
	}
	defer fence.Close()
	root := fence.canonicalRoot
	attemptID, lineageID := issueopsdomain.ManualCmuxHandoffIDs(request)
	observations, err := deps.ReadAudit()
	if err != nil {
		return result, err
	}
	if err := issueopsdomain.ValidateCmuxAttempt(request, observations); err != nil {
		return result, err
	}
	promptPath, err := issueopsdomain.CmuxCanonicalPromptPath(root, request.PromptFile, record.Execution.Workspace.Root, record.WorktreePath)
	if err != nil {
		return result, err
	}
	prompt, err := deps.ReadPrompt(root, promptPath, request.PromptSHA256)
	if err != nil {
		return result, err
	}
	if !issueopsdomain.ValidCmuxDigest(request.MaterialSHA256) {
		return result, fmt.Errorf("cmux handoff material digest is invalid")
	}
	if err := deps.ValidateHostExecutable(request.HostExecutable); err != nil {
		return result, err
	}
	if err := deps.ValidateHostProfile(request.Host, request.HostExecutable, request.Model, request.Effort); err != nil {
		return result, err
	}

	preflightRequest := cmuxcontract.PreflightRequest{
		Executable: request.CmuxExecutable, ExpectedVersion: request.CmuxVersion, ExpectedBuild: request.CmuxBuild,
		SocketPath: request.SocketPath, WindowID: request.WindowID,
	}
	if record, err = fence.Check(); err != nil {
		return result, err
	}
	preflightStarted := time.Now()
	preflight, err := deps.Client.Preflight(ctx, preflightRequest)
	if err != nil {
		result.Status = "unavailable"
		return result, err
	}
	preflightMS := cmuxElapsedMS(preflightStarted)
	endpoint := preflight.Endpoint
	createdAt := deps.Now().UTC().Format(time.RFC3339Nano)
	observation := issueopscontract.IssueOpsHandoffDeliveryObservation{
		SchemaVersion: issueopscontract.IssueOpsHandoffDeliverySchemaVersion,
		AttemptID:     attemptID, LineageID: lineageID, LifecycleID: request.ID,
		PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
		Launcher: issueopscontract.IssueOpsHandoffDeliveryLauncher{
			Name: issueopscontract.IssueOpsHandoffDeliveryLauncherCmux, Version: preflight.Build,
			Path: preflight.Executable, EndpointIncarnation: &endpoint,
		},
		Target:            issueopscontract.IssueOpsHandoffDeliveryTarget{WindowID: request.WindowID, CWD: root},
		ExpectedOwnerHost: request.Host, SourceGeneration: request.Generation,
		CreatedAt: createdAt, UpdatedAt: createdAt,
		CallStaged:    issueopsdomain.CmuxObserved(createdAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged),
		InputAccepted: issueopsdomain.CmuxNotObserved(), NativeTurnObserved: issueopsdomain.CmuxNotObserved(), OwnerClaimed: issueopsdomain.CmuxNotObserved(), Ambiguous: issueopsdomain.CmuxNotObserved(),
		Timing: &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: preflightMS},
	}
	audited, err := deps.Observe(record, observation)
	if err != nil {
		return result, err
	}
	observation = audited.Observation
	result = issueopsdomain.CmuxResult(request, observation, "staged", "")

	createRequest := cmuxcontract.CreateRequest{Preflight: preflight, AttemptID: attemptID, CWD: root}
	if record, err = fence.Check(); err != nil {
		return result, err
	}
	created, createErr := deps.Client.CreateWorkspace(ctx, createRequest)
	if created.CWD != "" && !deps.SamePath(created.CWD, root) {
		createErr = errors.Join(createErr, fmt.Errorf("cmux created workspace cwd target mismatch"))
	}
	observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	if created.WorkspaceID != "" {
		observation.Target.WorkspaceID = created.WorkspaceID
	}
	if created.SurfaceID != "" {
		observation.Target.SurfaceID = created.SurfaceID
	}
	observation.Timing.WorkspaceCreateMS = created.CreateMS
	observation.Timing.TargetResolveMS = created.ResolveMS
	if createErr != nil {
		if cmuxMutationAmbiguous(createErr) {
			observation.Ambiguous = issueopsdomain.CmuxObserved(observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
		}
		audited, auditErr := deps.Observe(record, observation)
		if auditErr != nil {
			return result, errors.Join(createErr, auditErr)
		}
		status := "failed"
		if cmuxMutationAmbiguous(createErr) {
			status = "ambiguous"
		}
		return issueopsdomain.CmuxResult(request, audited.Observation, status, ""), createErr
	}
	if observation.Target.WorkspaceID == "" || observation.Target.SurfaceID == "" {
		terminal, auditErr := deps.Observe(record, observation)
		return issueopsdomain.CmuxResult(request, terminal.Observation, "failed", ""), errors.Join(fmt.Errorf("cmux created target identity is incomplete"), auditErr)
	}
	audited, err = deps.Observe(record, observation)
	if err != nil {
		return result, err
	}
	observation = audited.Observation

	artifactRoot := deps.ArtifactRoot
	prepared, err := deps.Prepare(cmuxcontract.ArtifactRequest{
		Root: artifactRoot, CWD: root, WindowID: request.WindowID, WorkspaceID: created.WorkspaceID,
		SurfaceID: created.SurfaceID, SocketPath: request.SocketPath, Host: request.Host,
		HostExecutable: request.HostExecutable, Model: request.Model, Effort: request.Effort,
		Prompt: prompt, PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
	})
	if err != nil {
		observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
		audited, auditErr := deps.Observe(record, observation)
		return issueopsdomain.CmuxResult(request, audited.Observation, "pre_send_failed", ""), errors.Join(err, auditErr)
	}
	sendRequest := cmuxcontract.SendRequest{Created: created, Command: prepared.Command}
	checkedRecord, fenceErr := fence.Check()
	if fenceErr != nil {
		observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
		audited, auditErr := deps.Observe(record, observation)
		return issueopsdomain.CmuxResult(request, audited.Observation, "pre_send_failed", prepared.Directory), errors.Join(fenceErr, auditErr)
	}
	record = checkedRecord
	sendReceipt, sendErr := deps.Client.Send(ctx, sendRequest)
	observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	observation.Timing.InputSendMS = sendReceipt.SendMS
	if sendErr != nil {
		if cmuxMutationAmbiguous(sendErr) {
			observation.Ambiguous = issueopsdomain.CmuxObserved(observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
		}
		audited, auditErr := deps.Observe(record, observation)
		status := "failed"
		if cmuxMutationAmbiguous(sendErr) {
			status = "ambiguous"
		}
		return issueopsdomain.CmuxResult(request, audited.Observation, status, prepared.Directory), errors.Join(sendErr, auditErr)
	}
	if !sendReceipt.Accepted {
		audited, auditErr := deps.Observe(record, observation)
		return issueopsdomain.CmuxResult(request, audited.Observation, "failed", prepared.Directory), errors.Join(fmt.Errorf("cmux send returned no accepted receipt"), auditErr)
	}
	observation.InputAccepted = issueopsdomain.CmuxObserved(observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput)
	audited, err = deps.Observe(record, observation)
	if err != nil {
		return issueopsdomain.CmuxResult(request, observation, "input_accepted", prepared.Directory), err
	}
	observation = audited.Observation

	receiptStarted := time.Now()
	process, receiptErr := deps.AwaitReceipt(ctx, prepared, cmuxcontract.BootstrapExpectation{
		CWD: root, WindowID: created.WindowID, WorkspaceID: created.WorkspaceID, SurfaceID: created.SurfaceID, SocketPath: request.SocketPath,
		HostExecutable: request.HostExecutable, HostArgvSHA256: prepared.HostArgvSHA256,
		PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
	})
	observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	if receiptErr != nil {
		audited, auditErr := deps.Observe(record, observation)
		if auditErr != nil {
			return issueopsdomain.CmuxResult(request, observation, "input_accepted_receiver_unverified", prepared.Directory), auditErr
		}
		return issueopsdomain.CmuxResult(request, audited.Observation, "input_accepted_receiver_unverified", prepared.Directory), nil
	}
	observation.Target.Process = &process
	observation.Target.ProcessIncarnation = strconv.Itoa(process.PID) + ":" + process.StartedAt + ":" + process.Executable
	observation.Timing.ReceiverReceiptMS = cmuxElapsedMS(receiptStarted)
	audited, err = deps.Observe(record, observation)
	if err != nil {
		return issueopsdomain.CmuxResult(request, observation, "input_accepted", prepared.Directory), err
	}
	if cleanupErr := deps.Cleanup(prepared); cleanupErr != nil {
		return issueopsdomain.CmuxResult(request, audited.Observation, "input_accepted_cleanup_failed", prepared.Directory), fmt.Errorf("cleanup cmux recovery artifact: %w", cleanupErr)
	}
	return issueopsdomain.CmuxResult(request, audited.Observation, "input_accepted", ""), nil
}

func cmuxElapsedMS(start time.Time) uint64 {
	value := uint64(time.Since(start).Milliseconds())
	if value == 0 {
		return 1
	}
	return value
}

func cmuxMutationAmbiguous(err error) bool {
	mutation, ok := errors.AsType[*port.MutationError](err)
	return ok && mutation.Ambiguous
}
