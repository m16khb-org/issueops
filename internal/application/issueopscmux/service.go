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

// handoffRun carries the state shared by the ordered phases of one cmux handoff.
// Each phase records the result it would have returned on failure in run.result.
type handoffRun struct {
	deps        Service
	ctx         context.Context
	request     issueopscontract.ExecutionCmuxHandoffRequest
	fence       *rootFence
	record      issueopscontract.IssueOpsRecord
	root        string
	attemptID   string
	lineageID   string
	prompt      []byte
	result      issueopscontract.ExecutionCmuxHandoffResult
	preflight   cmuxcontract.PreflightResult
	preflightMS uint64
	observation issueopscontract.IssueOpsHandoffDeliveryObservation
	created     cmuxcontract.CreatedWorkspace
	prepared    cmuxcontract.PreparedLauncher
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
	run := &handoffRun{deps: deps, ctx: ctx, request: request, fence: fence, record: record, root: fence.canonicalRoot, result: result}
	run.attemptID, run.lineageID = issueopsdomain.ManualCmuxHandoffIDs(request)
	for _, phase := range []func() error{
		run.validateRequest, run.runPreflight, run.stageObservation, run.createWorkspace, run.prepareLauncher, run.sendLauncher,
	} {
		if err := phase(); err != nil {
			return run.result, err
		}
	}
	return run.confirmReceipt()
}

func (run *handoffRun) nowRFC3339() string {
	return run.deps.Now().UTC().Format(time.RFC3339Nano)
}

// validateRequest checks the attempt history, prompt, digests, and host profile before any external call.
func (run *handoffRun) validateRequest() error {
	request := run.request
	observations, err := run.deps.ReadAudit()
	if err != nil {
		return err
	}
	if err := issueopsdomain.ValidateCmuxAttempt(request, observations); err != nil {
		return err
	}
	promptPath, err := issueopsdomain.CmuxCanonicalPromptPath(run.root, request.PromptFile, run.record.Execution.Workspace.Root, run.record.WorktreePath)
	if err != nil {
		return err
	}
	run.prompt, err = run.deps.ReadPrompt(run.root, promptPath, request.PromptSHA256)
	if err != nil {
		return err
	}
	if !issueopsdomain.ValidCmuxDigest(request.MaterialSHA256) {
		return fmt.Errorf("cmux handoff material digest is invalid")
	}
	if err := run.deps.ValidateHostExecutable(request.HostExecutable); err != nil {
		return err
	}
	return run.deps.ValidateHostProfile(request.Host, request.HostExecutable, request.Model, request.Effort)
}

func (run *handoffRun) runPreflight() error {
	request := run.request
	preflightRequest := cmuxcontract.PreflightRequest{
		Executable: request.CmuxExecutable, ExpectedVersion: request.CmuxVersion, ExpectedBuild: request.CmuxBuild,
		SocketPath: request.SocketPath, WindowID: request.WindowID,
	}
	var err error
	if run.record, err = run.fence.Check(); err != nil {
		return err
	}
	preflightStarted := time.Now()
	run.preflight, err = run.deps.Client.Preflight(run.ctx, preflightRequest)
	if err != nil {
		run.result.Status = "unavailable"
		return err
	}
	run.preflightMS = cmuxElapsedMS(preflightStarted)
	return nil
}

// stageObservation records the external-call-staged audit entry.
func (run *handoffRun) stageObservation() error {
	request := run.request
	endpoint := run.preflight.Endpoint
	createdAt := run.nowRFC3339()
	observation := issueopscontract.IssueOpsHandoffDeliveryObservation{
		SchemaVersion: issueopscontract.IssueOpsHandoffDeliverySchemaVersion,
		AttemptID:     run.attemptID, LineageID: run.lineageID, LifecycleID: request.ID,
		PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
		Launcher: issueopscontract.IssueOpsHandoffDeliveryLauncher{
			Name: issueopscontract.IssueOpsHandoffDeliveryLauncherCmux, Version: run.preflight.Build,
			Path: run.preflight.Executable, EndpointIncarnation: &endpoint,
		},
		Target:            issueopscontract.IssueOpsHandoffDeliveryTarget{WindowID: request.WindowID, CWD: run.root},
		ExpectedOwnerHost: request.Host, SourceGeneration: request.Generation,
		CreatedAt: createdAt, UpdatedAt: createdAt,
		CallStaged:    issueopsdomain.CmuxObserved(createdAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged),
		InputAccepted: issueopsdomain.CmuxNotObserved(), NativeTurnObserved: issueopsdomain.CmuxNotObserved(), OwnerClaimed: issueopsdomain.CmuxNotObserved(), Ambiguous: issueopsdomain.CmuxNotObserved(),
		Timing: &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: run.preflightMS},
	}
	audited, err := run.deps.Observe(run.record, observation)
	if err != nil {
		return err
	}
	run.observation = audited.Observation
	run.result = issueopsdomain.CmuxResult(request, run.observation, "staged", "")
	return nil
}

// observe audits the current observation and adopts the audited copy.
func (run *handoffRun) observe() error {
	audited, err := run.deps.Observe(run.record, run.observation)
	if err != nil {
		return err
	}
	run.observation = audited.Observation
	return nil
}

// markAmbiguousIfLost flags the observation when a mutation may have been accepted without a response.
func (run *handoffRun) markAmbiguousIfLost(err error) {
	if cmuxMutationAmbiguous(err) {
		run.observation.Ambiguous = issueopsdomain.CmuxObserved(run.observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
	}
}

func cmuxMutationFailureStatus(err error) string {
	if cmuxMutationAmbiguous(err) {
		return "ambiguous"
	}
	return "failed"
}

// failWithAudit audits the current observation, sets the failure result, and returns cause joined with any audit error.
func (run *handoffRun) failWithAudit(status, recovery string, cause error) error {
	audited, auditErr := run.deps.Observe(run.record, run.observation)
	run.result = issueopsdomain.CmuxResult(run.request, audited.Observation, status, recovery)
	return errors.Join(cause, auditErr)
}

func (run *handoffRun) createWorkspace() error {
	var err error
	if run.record, err = run.fence.Check(); err != nil {
		return err
	}
	created, createErr := run.deps.Client.CreateWorkspace(run.ctx, cmuxcontract.CreateRequest{Preflight: run.preflight, AttemptID: run.attemptID, CWD: run.root})
	if created.CWD != "" && !run.deps.SamePath(created.CWD, run.root) {
		createErr = errors.Join(createErr, fmt.Errorf("cmux created workspace cwd target mismatch"))
	}
	run.created = created
	run.observation.UpdatedAt = run.nowRFC3339()
	if created.WorkspaceID != "" {
		run.observation.Target.WorkspaceID = created.WorkspaceID
	}
	if created.SurfaceID != "" {
		run.observation.Target.SurfaceID = created.SurfaceID
	}
	run.observation.Timing.WorkspaceCreateMS = created.CreateMS
	run.observation.Timing.TargetResolveMS = created.ResolveMS
	if createErr != nil {
		run.markAmbiguousIfLost(createErr)
		audited, auditErr := run.deps.Observe(run.record, run.observation)
		if auditErr != nil {
			return errors.Join(createErr, auditErr)
		}
		run.result = issueopsdomain.CmuxResult(run.request, audited.Observation, cmuxMutationFailureStatus(createErr), "")
		return createErr
	}
	if run.observation.Target.WorkspaceID == "" || run.observation.Target.SurfaceID == "" {
		return run.failWithAudit("failed", "", fmt.Errorf("cmux created target identity is incomplete"))
	}
	return run.observe()
}

func (run *handoffRun) prepareLauncher() error {
	request := run.request
	prepared, err := run.deps.Prepare(cmuxcontract.ArtifactRequest{
		Root: run.deps.ArtifactRoot, CWD: run.root, WindowID: request.WindowID, WorkspaceID: run.created.WorkspaceID,
		SurfaceID: run.created.SurfaceID, SocketPath: request.SocketPath, Host: request.Host,
		HostExecutable: request.HostExecutable, Model: request.Model, Effort: request.Effort,
		Prompt: run.prompt, PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
	})
	if err != nil {
		run.observation.UpdatedAt = run.nowRFC3339()
		return run.failWithAudit("pre_send_failed", "", err)
	}
	run.prepared = prepared
	return nil
}

// sendLauncher sends the prepared launcher command and records whether the input was accepted.
func (run *handoffRun) sendLauncher() error {
	checkedRecord, fenceErr := run.fence.Check()
	if fenceErr != nil {
		run.observation.UpdatedAt = run.nowRFC3339()
		return run.failWithAudit("pre_send_failed", run.prepared.Directory, fenceErr)
	}
	run.record = checkedRecord
	sendReceipt, sendErr := run.deps.Client.Send(run.ctx, cmuxcontract.SendRequest{Created: run.created, Command: run.prepared.Command})
	run.observation.UpdatedAt = run.nowRFC3339()
	run.observation.Timing.InputSendMS = sendReceipt.SendMS
	if sendErr != nil {
		run.markAmbiguousIfLost(sendErr)
		return run.failWithAudit(cmuxMutationFailureStatus(sendErr), run.prepared.Directory, sendErr)
	}
	if !sendReceipt.Accepted {
		return run.failWithAudit("failed", run.prepared.Directory, fmt.Errorf("cmux send returned no accepted receipt"))
	}
	run.observation.InputAccepted = issueopsdomain.CmuxObserved(run.observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput)
	if err := run.observe(); err != nil {
		run.result = issueopsdomain.CmuxResult(run.request, run.observation, "input_accepted", run.prepared.Directory)
		return err
	}
	return nil
}

// confirmReceipt waits for the launcher's bootstrap receipt, records the process, and cleans up the recovery artifact.
func (run *handoffRun) confirmReceipt() (issueopscontract.ExecutionCmuxHandoffResult, error) {
	request, created, prepared := run.request, run.created, run.prepared
	receiptStarted := time.Now()
	process, receiptErr := run.deps.AwaitReceipt(run.ctx, prepared, cmuxcontract.BootstrapExpectation{
		CWD: run.root, WindowID: created.WindowID, WorkspaceID: created.WorkspaceID, SurfaceID: created.SurfaceID, SocketPath: request.SocketPath,
		HostExecutable: request.HostExecutable, HostArgvSHA256: prepared.HostArgvSHA256,
		PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
	})
	run.observation.UpdatedAt = run.nowRFC3339()
	if receiptErr != nil {
		audited, auditErr := run.deps.Observe(run.record, run.observation)
		if auditErr != nil {
			return issueopsdomain.CmuxResult(request, run.observation, "input_accepted_receiver_unverified", prepared.Directory), auditErr
		}
		return issueopsdomain.CmuxResult(request, audited.Observation, "input_accepted_receiver_unverified", prepared.Directory), nil
	}
	run.observation.Target.Process = &process
	run.observation.Target.ProcessIncarnation = strconv.Itoa(process.PID) + ":" + process.StartedAt + ":" + process.Executable
	run.observation.Timing.ReceiverReceiptMS = cmuxElapsedMS(receiptStarted)
	audited, err := run.deps.Observe(run.record, run.observation)
	if err != nil {
		return issueopsdomain.CmuxResult(request, run.observation, "input_accepted", prepared.Directory), err
	}
	if cleanupErr := run.deps.Cleanup(prepared); cleanupErr != nil {
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
