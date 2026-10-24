package issueopspreparation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

type Service struct {
	repository Repository
	clock      Clock
	direct     DirectWorkspace
	evidence   PreparationEvidence
}

func NewService(repository Repository, clock Clock, direct DirectWorkspace, evidence PreparationEvidence) *Service {
	return &Service{repository: repository, clock: clock, direct: direct, evidence: evidence}
}

func (service *Service) Prepare(ctx context.Context, command preparationcontract.Command) (preparationcontract.Result, error) {
	if service.repository == nil {
		return failedResult(command.ID), fmt.Errorf("preparation repository is unavailable")
	}
	snapshot, err := service.repository.Load(ctx, command.ID)
	if err != nil {
		return failedResult(command.ID), err
	}
	decision := preparationdomain.Decide(preparationdomain.DecisionInput{Command: command, Snapshot: snapshot})
	if snapshot.Record.Execution != nil {
		return service.resolveExisting(snapshot, command, decision)
	}
	if service.evidence == nil {
		return failedResult(command.ID), fmt.Errorf("preparation evidence is unavailable")
	}
	workspace, err := service.evidence.Workspace(snapshot, command.Confirm)
	if err != nil {
		return failedResult(command.ID), err
	}
	snapshot.CanonicalRoot = workspace.Root
	if err := service.repository.EnsureRootUnclaimed(ctx, command.ID, workspace.Root); err != nil {
		return failedResult(command.ID), err
	}
	return service.prepareDirect(ctx, snapshot, command, workspace)
}

func (service *Service) prepareDirect(ctx context.Context, snapshot preparationcontract.Snapshot, command preparationcontract.Command, workspace preparationcontract.WorkspaceRequest) (preparationcontract.Result, error) {
	actor, err := normalizeActor(command.Actor)
	if err != nil {
		return failedResult(command.ID), err
	}
	command.Actor = actor
	// Reject before provisioning, in preview too: a session holding another
	// cycle's lease would otherwise fail at commit with a worktree left behind.
	if err := service.repository.EnsureActorHoldsNoLease(ctx, actor); err != nil {
		return failedResult(command.ID), err
	}
	workspace.CWD = command.CWD
	// Reject before provisioning so a URL without an issue number cannot leave
	// a worktree behind that CommitDirect would refuse.
	if err := requireArtifactDir(snapshot.ArtifactDir); err != nil {
		return failedResult(command.ID), err
	}
	if service.direct == nil {
		return failedResult(command.ID), fmt.Errorf("direct Git worktree provisioner is unavailable")
	}
	if command.Confirm {
		access, err := service.direct.ProbeAccess(ctx, workspace, actor.Host)
		if err != nil {
			return failedResult(command.ID), err
		}
		if !access.Allowed {
			return preparationcontract.Result{
				ID: command.ID, Workspace: workspaceResult(workspace, ""), NextCommand: access.RelaunchCommand,
			}, fmt.Errorf("canonical worktree base is not accessible; relaunch with: %s", access.RelaunchCommand)
		}
	}
	receipt, err := service.direct.Prepare(ctx, workspace)
	if err != nil {
		return failedResult(command.ID), err
	}
	if service.clock == nil {
		return failedResult(command.ID), fmt.Errorf("preparation clock is unavailable")
	}
	linkedAt := formatTime(service.clock.Now())
	result := preparationcontract.Result{
		OK: true, ID: command.ID, Preview: !command.Confirm,
		Workspace: workspaceResultFromReceipt(receipt, linkedAt),
	}
	if !command.Confirm {
		result.NextCommand = prepareConfirmCommand(command)
		return result, nil
	}
	claimedAt := formatTime(service.clock.Now())
	if err := service.evidence.MaterializeDirect(ctx, snapshot, receipt); err != nil {
		return failedResult(command.ID), err
	}
	persisted, err := service.repository.CommitDirect(ctx, DirectCommit{
		Snapshot: snapshot, Command: command, Workspace: receipt,
		LinkedAt: linkedAt, ClaimedAt: claimedAt,
	})
	if err != nil {
		return failedResult(command.ID), err
	}
	return persisted, nil
}

func (service *Service) resolveExisting(snapshot preparationcontract.Snapshot, command preparationcontract.Command, decision preparationdomain.Decision) (preparationcontract.Result, error) {
	result := preparedResult(snapshot.Record)
	switch decision.Code {
	case preparationdomain.CodeExisting:
		return result, nil
	case preparationdomain.CodePendingReconcile:
		result.OK = false
		result.NextCommand = "issueops execution reconcile --id " + snapshot.Record.ID + " --preview ACTOR_FLAGS"
		return result, fmt.Errorf("IssueOps execution has a pending external intent; run %s", result.NextCommand)
	case preparationdomain.CodeWriterless:
		result.OK = false
		result.NextCommand = writerlessNextCommand(snapshot)
		return result, writerlessError(snapshot.Record, result.NextCommand)
	default:
		return failedResult(command.ID), fmt.Errorf("unsupported preparation decision %q", decision.Code)
	}
}

func normalizeActor(actor leasecontract.Actor) (leasecontract.Actor, error) {
	actor.Host = strings.ToLower(strings.TrimSpace(actor.Host))
	actor.SessionID = strings.TrimSpace(actor.SessionID)
	actor.AgentID = strings.TrimSpace(actor.AgentID)
	if actor.SessionProcess != nil {
		process := *actor.SessionProcess
		process.StartedAt = strings.TrimSpace(process.StartedAt)
		process.Executable = strings.TrimSpace(process.Executable)
		actor.SessionProcess = &process
	}
	if actor.Host != "codex" && actor.Host != "claude" && actor.Host != "omo" && actor.Host != "omp" && actor.Host != "opencode" {
		return actor, fmt.Errorf("native actor host must be codex, claude, omo, omp, or opencode")
	}
	if actor.SessionID == "" {
		return actor, fmt.Errorf("native actor session_id is required")
	}
	if actor.SessionProcess == nil || actor.SessionProcess.PID <= 0 || actor.SessionProcess.StartedAt == "" || actor.SessionProcess.Executable == "" {
		return actor, fmt.Errorf("native actor requires a PID reuse-safe session_process receipt")
	}
	return actor, nil
}

func preparedResult(record preparationcontract.Record) preparationcontract.Result {
	return preparationcontract.Result{
		OK: true, ID: record.ID, Workspace: record.Execution.Workspace, Execution: cloneExecution(record.Execution),
	}
}

func prepareConfirmCommand(command preparationcontract.Command) string {
	parts := []string{"issueops", "execution", "prepare", "--id", quoteArg(command.ID)}
	if command.Actor.Host != "" {
		parts = append(parts, "--host", quoteArg(command.Actor.Host))
	}
	if command.Actor.SessionID != "" {
		parts = append(parts, "--session-id", quoteArg(command.Actor.SessionID))
	}
	if command.Actor.AgentID != "" {
		parts = append(parts, "--agent-id", quoteArg(command.Actor.AgentID))
	}
	if command.Actor.SessionProcess != nil {
		parts = append(parts, "--session-pid", strconv.Itoa(command.Actor.SessionProcess.PID), "--session-started-at", quoteArg(command.Actor.SessionProcess.StartedAt), "--session-executable", quoteArg(command.Actor.SessionProcess.Executable))
	}
	if command.CWD != "" {
		parts = append(parts, "--cwd", quoteArg(command.CWD))
	}
	return strings.Join(append(parts, "--confirm", "--json"), " ")
}

func cloneExecution(execution *leasecontract.Execution) *leasecontract.Execution {
	if execution == nil {
		return nil
	}
	return preparationcontract.Result{Execution: execution}.Clone().Execution
}

func workspaceResult(request preparationcontract.WorkspaceRequest, linkedAt string) leasecontract.Workspace {
	return leasecontract.Workspace{SourceRoot: request.SourceRoot, Root: request.Root, Branch: request.Branch, BaseHead: request.BaseHead, ParentWorktree: request.ParentWorktree, LinkedAt: linkedAt}
}

func workspaceResultFromReceipt(receipt preparationcontract.WorkspaceReceipt, linkedAt string) leasecontract.Workspace {
	return leasecontract.Workspace{SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, LinkedAt: linkedAt}
}

func writerlessNextCommand(snapshot preparationcontract.Snapshot) string {
	record := snapshot.Record
	generation := record.Execution.Lease.Generation
	switch record.Execution.Lease.Status {
	case "claimable":
		return "issueops execution claim --id " + quoteArg(record.ID) + " --generation " + strconv.FormatUint(generation, 10) + " --claim-current-token"
	case "released":
		return "issueops execution replace --id " + quoteArg(record.ID) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
	case "revoking":
		return "issueops execution replace --id " + quoteArg(record.ID) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --finalize-preview"
	default:
		return ""
	}
}

func writerlessError(record preparationcontract.Record, next string) error {
	lease := record.Execution.Lease
	switch lease.Status {
	case "claimable":
		return fmt.Errorf("IssueOps execution is prepared but generation %d is claimable and has no writer; run %s", lease.Generation, next)
	case "released":
		return fmt.Errorf("IssueOps execution is prepared but generation %d was released and has no writer; preview replacement with %s", lease.Generation, next)
	case "revoking":
		return fmt.Errorf("IssueOps execution generation %d is revoking and has no writer; finalize the revocation with %s", lease.Generation, next)
	default:
		return errors.New("IssueOps execution has no writer")
	}
}

func quoteArg(value string) string                      { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
func formatTime(value time.Time) string                 { return value.UTC().Format(time.RFC3339Nano) }
func failedResult(id string) preparationcontract.Result { return preparationcontract.Result{ID: id} }
