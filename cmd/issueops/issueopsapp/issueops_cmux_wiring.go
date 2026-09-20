package issueopsapp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	auditadapter "issueops/internal/adapter/audit"
	cmuxadapter "issueops/internal/adapter/cmux"
	issueopsadapter "issueops/internal/adapter/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/nativehost"
)

type cmuxClient interface {
	Preflight(context.Context, cmuxadapter.PreflightRequest) (cmuxadapter.PreflightResult, error)
	CreateWorkspace(context.Context, cmuxadapter.CreateRequest) (cmuxadapter.CreatedWorkspace, error)
	Send(context.Context, cmuxadapter.SendRequest) (cmuxadapter.SendReceipt, error)
}

type cmuxHandoffDependencies struct {
	Client                 cmuxClient
	Now                    func() time.Time
	Getwd                  func() (string, error)
	GitTop                 func(string) (string, error)
	ReadPrompt             func(string, string, string) ([]byte, error)
	ValidateHostExecutable func(string) error
	Prepare                func(cmuxadapter.ArtifactRequest) (cmuxadapter.PreparedLauncher, error)
	AwaitReceipt           func(context.Context, cmuxadapter.PreparedLauncher, cmuxadapter.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error)
	Cleanup                func(cmuxadapter.PreparedLauncher) error
}

func issueOpsCmuxHandoffHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionCmuxHandoffRequest) (issueopscontract.ExecutionCmuxHandoffResult, error) {
	client := cmuxadapter.Client{Runner: cmuxadapter.ExecRunner{}, ObserveEndpoint: cmuxadapter.ObserveEndpoint, UID: os.Getuid()}
	return issueOpsCmuxHandoffHandlerWithDeps(ctx, stateRoot, request, cmuxHandoffDependencies{
		Client: client, Now: time.Now, Getwd: os.Getwd, GitTop: cmuxGitTop, ReadPrompt: cmuxadapter.ReadPrompt,
		ValidateHostExecutable: validateCmuxHostExecutable, Prepare: cmuxadapter.PrepareLauncher,
		AwaitReceipt: awaitCmuxBootstrapReceipt, Cleanup: func(prepared cmuxadapter.PreparedLauncher) error { return prepared.Cleanup() },
	})
}

func issueOpsCmuxHandoffHandlerWithDeps(ctx context.Context, stateRoot string, request issueopscontract.ExecutionCmuxHandoffRequest, deps cmuxHandoffDependencies) (issueopscontract.ExecutionCmuxHandoffResult, error) {
	result := issueopscontract.ExecutionCmuxHandoffResult{ID: request.ID, Generation: request.Generation, Status: "rejected"}
	if deps.Client == nil || deps.Now == nil || deps.GitTop == nil || deps.ValidateHostExecutable == nil {
		return result, fmt.Errorf("cmux handoff dependencies are unavailable")
	}
	if deps.Getwd == nil {
		deps.Getwd = os.Getwd
	}
	if deps.ReadPrompt == nil {
		deps.ReadPrompt = cmuxadapter.ReadPrompt
	}
	if cmuxRequestContainsNUL(request) {
		return result, fmt.Errorf("cmux handoff identity must not contain NUL bytes")
	}
	record, err := issueopsadapter.ReadIssueOps(stateRoot, request.ID)
	if err != nil {
		return result, err
	}
	root, err := validateCmuxReleasedDirectRecord(record, request.Generation, request.CWD, deps.Getwd, deps.GitTop)
	if err != nil {
		return result, err
	}
	attemptID, lineageID := cmuxHandoffIDs(request)
	observations, err := auditadapter.ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	if err != nil {
		return result, err
	}
	for _, observation := range observations {
		if observation.LifecycleID == request.ID && observation.LineageID == lineageID {
			return result, fmt.Errorf("cmux handoff attempt already exists; inspect the existing lineage and do not retry")
		}
		if observation.LifecycleID == request.ID && observation.SourceGeneration == request.Generation &&
			observation.Launcher.Name == issueopscontract.IssueOpsHandoffDeliveryLauncherCmux &&
			observation.CallStaged.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved {
			return result, fmt.Errorf("cmux handoff generation already has a staged attempt; inspect the existing lineage and do not retry")
		}
	}
	prompt, err := deps.ReadPrompt(root, request.PromptFile, request.PromptSHA256)
	if err != nil {
		return result, err
	}
	if !cmuxDigest(request.MaterialSHA256) {
		return result, fmt.Errorf("cmux handoff material digest is invalid")
	}
	if err := deps.ValidateHostExecutable(request.HostExecutable); err != nil {
		return result, err
	}
	if _, err := nativehost.BuildInteractiveArgv(request.Host, request.HostExecutable, request.Model, request.Effort, ""); err != nil {
		return result, err
	}

	preflightStarted := time.Now()
	preflight, err := deps.Client.Preflight(ctx, cmuxadapter.PreflightRequest{
		Executable: request.CmuxExecutable, ExpectedVersion: request.CmuxVersion, ExpectedBuild: request.CmuxBuild,
		SocketPath: request.SocketPath, WindowID: request.WindowID,
	})
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
		CallStaged:    cmuxObserved(createdAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged),
		InputAccepted: cmuxNotObserved(), NativeTurnObserved: cmuxNotObserved(), OwnerClaimed: cmuxNotObserved(), Ambiguous: cmuxNotObserved(),
		Timing: &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: preflightMS},
	}
	audited, err := auditCmuxObservation(stateRoot, record, observation)
	if err != nil {
		return result, err
	}
	observation = audited.Observation
	result = cmuxResult(request, observation, "staged", "")

	created, createErr := deps.Client.CreateWorkspace(ctx, cmuxadapter.CreateRequest{Preflight: preflight, AttemptID: attemptID, CWD: root})
	if created.CWD != "" && !sameCmuxPath(created.CWD, root) {
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
			observation.Ambiguous = cmuxObserved(observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
		}
		audited, auditErr := auditCmuxObservation(stateRoot, record, observation)
		if auditErr != nil {
			return result, errors.Join(createErr, auditErr)
		}
		status := "failed"
		if cmuxMutationAmbiguous(createErr) {
			status = "ambiguous"
		}
		return cmuxResult(request, audited.Observation, status, ""), createErr
	}
	if observation.Target.WorkspaceID == "" || observation.Target.SurfaceID == "" {
		terminal, auditErr := auditCmuxObservation(stateRoot, record, observation)
		return cmuxResult(request, terminal.Observation, "failed", ""), errors.Join(fmt.Errorf("cmux created target identity is incomplete"), auditErr)
	}
	audited, err = auditCmuxObservation(stateRoot, record, observation)
	if err != nil {
		return result, err
	}
	observation = audited.Observation

	if deps.Prepare == nil {
		deps.Prepare = cmuxadapter.PrepareLauncher
	}
	artifactRoot := filepath.Join(stateRoot, "cmux-handoff")
	prepared, err := deps.Prepare(cmuxadapter.ArtifactRequest{
		Root: artifactRoot, CWD: root, WindowID: request.WindowID, WorkspaceID: created.WorkspaceID,
		SurfaceID: created.SurfaceID, SocketPath: request.SocketPath, Host: request.Host,
		HostExecutable: request.HostExecutable, Model: request.Model, Effort: request.Effort,
		Prompt: prompt, PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
	})
	if err != nil {
		observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
		audited, auditErr := auditCmuxObservation(stateRoot, record, observation)
		return cmuxResult(request, audited.Observation, "pre_send_failed", ""), errors.Join(err, auditErr)
	}
	sendReceipt, sendErr := deps.Client.Send(ctx, cmuxadapter.SendRequest{Created: created, Command: prepared.Command})
	observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	observation.Timing.InputSendMS = sendReceipt.SendMS
	if sendErr != nil {
		if cmuxMutationAmbiguous(sendErr) {
			observation.Ambiguous = cmuxObserved(observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost)
		}
		audited, auditErr := auditCmuxObservation(stateRoot, record, observation)
		status := "failed"
		if cmuxMutationAmbiguous(sendErr) {
			status = "ambiguous"
		}
		return cmuxResult(request, audited.Observation, status, prepared.Directory), errors.Join(sendErr, auditErr)
	}
	if !sendReceipt.Accepted {
		audited, auditErr := auditCmuxObservation(stateRoot, record, observation)
		return cmuxResult(request, audited.Observation, "failed", prepared.Directory), errors.Join(fmt.Errorf("cmux send returned no accepted receipt"), auditErr)
	}
	observation.InputAccepted = cmuxObserved(observation.UpdatedAt, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput)
	audited, err = auditCmuxObservation(stateRoot, record, observation)
	if err != nil {
		return cmuxResult(request, observation, "input_accepted", prepared.Directory), err
	}
	observation = audited.Observation

	if deps.AwaitReceipt == nil {
		deps.AwaitReceipt = awaitCmuxBootstrapReceipt
	}
	receiptStarted := time.Now()
	process, receiptErr := deps.AwaitReceipt(ctx, prepared, cmuxadapter.BootstrapExpectation{
		CWD: root, WindowID: created.WindowID, WorkspaceID: created.WorkspaceID, SurfaceID: created.SurfaceID, SocketPath: request.SocketPath,
		HostExecutable: request.HostExecutable, HostArgvSHA256: prepared.HostArgvSHA256,
		PromptSHA256: request.PromptSHA256, MaterialSHA256: request.MaterialSHA256,
	})
	observation.UpdatedAt = deps.Now().UTC().Format(time.RFC3339Nano)
	if receiptErr != nil {
		audited, auditErr := auditCmuxObservation(stateRoot, record, observation)
		if auditErr != nil {
			return cmuxResult(request, observation, "input_accepted_receiver_unverified", prepared.Directory), auditErr
		}
		return cmuxResult(request, audited.Observation, "input_accepted_receiver_unverified", prepared.Directory), nil
	}
	observation.Target.Process = &process
	observation.Target.ProcessIncarnation = strconv.Itoa(process.PID) + ":" + process.StartedAt + ":" + process.Executable
	observation.Timing.ReceiverReceiptMS = cmuxElapsedMS(receiptStarted)
	audited, err = auditCmuxObservation(stateRoot, record, observation)
	if err != nil {
		return cmuxResult(request, observation, "input_accepted", prepared.Directory), err
	}
	if deps.Cleanup == nil {
		deps.Cleanup = func(prepared cmuxadapter.PreparedLauncher) error { return prepared.Cleanup() }
	}
	if cleanupErr := deps.Cleanup(prepared); cleanupErr != nil {
		return cmuxResult(request, audited.Observation, "input_accepted_cleanup_failed", prepared.Directory), fmt.Errorf("cleanup cmux recovery artifact: %w", cleanupErr)
	}
	return cmuxResult(request, audited.Observation, "input_accepted", ""), nil
}

func auditCmuxObservation(stateRoot string, record issueopscontract.IssueOpsRecord, observation issueopscontract.IssueOpsHandoffDeliveryObservation) (auditadapter.HandoffDeliveryAuditRecord, error) {
	if err := validateManualHandoffDeliveryObservation(record, observation); err != nil {
		return auditadapter.HandoffDeliveryAuditRecord{}, err
	}
	return auditadapter.AuditHandoffDeliveryObservationAt(stateRoot, observation)
}

func cmuxHandoffIDs(request issueopscontract.ExecutionCmuxHandoffRequest) (string, string) {
	identity := strings.Join([]string{request.ID, strconv.FormatUint(request.Generation, 10), request.WindowID, request.PromptSHA256, request.MaterialSHA256}, "\x00")
	sum := sha256.Sum256([]byte(identity))
	suffix := hex.EncodeToString(sum[:12])
	attempt := handoffDeliveryManualLineagePrefix + request.ID + ":" + strconv.FormatUint(request.Generation, 10) + ":cmux:" + suffix
	lineage := strings.Join([]string{handoffDeliveryManualLineagePrefix + "generation", strconv.FormatUint(request.Generation, 10), "cmux", "window", request.WindowID, "prompt", request.PromptSHA256, "material", request.MaterialSHA256}, ":")
	return attempt, lineage
}

func validateCmuxReleasedDirectRecord(record issueopscontract.IssueOpsRecord, generation uint64, requestedCWD string, getwd func() (string, error), gitTop func(string) (string, error)) (string, error) {
	if record.Execution == nil || record.Execution.Mode != issueopscontract.ExecutionModeDirect ||
		record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased || record.Execution.Lease.Generation != generation {
		return "", fmt.Errorf("cmux handoff requires the exact released direct execution generation")
	}
	root := filepath.Clean(record.Execution.Workspace.Root)
	if !filepath.IsAbs(root) || !sameCmuxPath(root, record.WorktreePath) {
		return "", fmt.Errorf("cmux handoff canonical worktree identity mismatch")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("cmux handoff canonical worktree is unavailable")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(requestedCWD) || filepath.Clean(requestedCWD) != requestedCWD || !sameCmuxPath(requestedCWD, resolvedRoot) {
		return "", fmt.Errorf("cmux handoff request cwd is not the canonical worktree")
	}
	processCWD, err := getwd()
	if err != nil || !filepath.IsAbs(processCWD) || !sameCmuxPath(processCWD, resolvedRoot) {
		return "", fmt.Errorf("cmux handoff process cwd is not the canonical worktree")
	}
	top, err := gitTop(resolvedRoot)
	if err != nil || !sameCmuxPath(top, root) {
		return "", fmt.Errorf("cmux handoff cwd is not the canonical Git worktree")
	}
	return root, nil
}

func validateCmuxHostExecutable(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("native host executable must be an absolute clean path")
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("native host executable is unavailable")
	}
	return nil
}

func cmuxGitTop(root string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--show-toplevel").Output()
	return strings.TrimSpace(string(output)), err
}

func awaitCmuxBootstrapReceipt(ctx context.Context, prepared cmuxadapter.PreparedLauncher, expected cmuxadapter.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error) {
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := cmuxadapter.ReadBootstrapReceipt(prepared.ReceiptPath)
		if err == nil {
			process, validationErr := cmuxadapter.ValidateBootstrapReceipt(receipt, expected, issueopsadapter.ObserveNativeProcessReceipt)
			if validationErr == nil && !cmuxShellProcess(process.Executable) {
				return process, nil
			}
			if validationErr != nil && receipt.Status != "ok" {
				return issueopscontract.NativeProcessReceipt{}, validationErr
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return issueopscontract.NativeProcessReceipt{}, err
		}
		select {
		case <-ctx.Done():
			return issueopscontract.NativeProcessReceipt{}, ctx.Err()
		case <-timer.C:
			return issueopscontract.NativeProcessReceipt{}, fmt.Errorf("cmux receiver bootstrap receipt timed out")
		case <-ticker.C:
		}
	}
}

func cmuxResult(request issueopscontract.ExecutionCmuxHandoffRequest, observation issueopscontract.IssueOpsHandoffDeliveryObservation, status, recovery string) issueopscontract.ExecutionCmuxHandoffResult {
	return issueopscontract.ExecutionCmuxHandoffResult{
		OK: observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved &&
			observation.Ambiguous.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		ID: request.ID, Generation: request.Generation, Status: status,
		Launcher: observation.Launcher, Target: observation.Target, Timing: observation.Timing,
		InputAccepted:       observation.InputAccepted.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		NativeTurnObserved:  observation.NativeTurnObserved.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		OwnerClaimed:        observation.OwnerClaimed.Status == issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		RecoveryArtifactDir: recovery, ObservationReceipt: observation.Receipt,
	}
}

func cmuxObserved(at, evidence string) issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: at, Evidence: evidence}
}

func cmuxNotObserved() issueopscontract.IssueOpsHandoffDeliveryState {
	return issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved}
}

func sameCmuxPath(left, right string) bool {
	leftResolved, leftErr := filepath.EvalSymlinks(filepath.Clean(left))
	rightResolved, rightErr := filepath.EvalSymlinks(filepath.Clean(right))
	return leftErr == nil && rightErr == nil && leftResolved == rightResolved
}

func cmuxDigest(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func cmuxElapsedMS(start time.Time) uint64 {
	value := uint64(time.Since(start).Milliseconds())
	if value == 0 {
		return 1
	}
	return value
}

func cmuxShellProcess(executable string) bool {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(executable)), ".exe")
	return base == "sh" || base == "bash" || base == "zsh" || base == "dash"
}

func cmuxMutationAmbiguous(err error) bool {
	mutation, ok := errors.AsType[*cmuxadapter.MutationError](err)
	return ok && mutation.Ambiguous
}

func cmuxRequestContainsNUL(request issueopscontract.ExecutionCmuxHandoffRequest) bool {
	for _, value := range []string{
		request.ID, request.CmuxExecutable, request.CmuxVersion, request.CmuxBuild, request.SocketPath,
		request.WindowID, request.CWD, request.Host, request.HostExecutable, request.Model, request.Effort,
		request.PromptFile, request.PromptSHA256, request.MaterialSHA256,
	} {
		if strings.ContainsRune(value, 0) {
			return true
		}
	}
	return false
}
