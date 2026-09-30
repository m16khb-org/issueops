package issueopspreparation

import (
	"fmt"
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type ResumeIntentInput struct {
	Record              preparationcontract.Record
	Workspace           preparationcontract.WorkspaceRequest
	Artifacts           preparationcontract.ResumeArtifacts
	RuntimeID           string
	ReusedTerminalPTYID string
	OperationID         string
	StartedAt           string
}

func BuildResumeIntent(input ResumeIntentInput) (preparationcontract.Intent, error) {
	record := input.Record
	if record.Execution == nil || record.Execution.Orca == nil {
		return preparationcontract.Intent{}, fmt.Errorf("execution resume requires an existing Orca binding")
	}
	issue, err := PrepareIssueIdentity(record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(record.BranchPrepare))
	if err != nil {
		return preparationcontract.Intent{}, err
	}
	execution := record.Execution
	binding, lease := execution.Orca, execution.Lease
	stage := preparationcontract.IntentStageTerminal
	if input.ReusedTerminalPTYID != "" {
		stage = preparationcontract.IntentStageRun
	}
	intent := preparationcontract.Intent{
		SchemaVersion: preparationcontract.SchemaVersion, Purpose: preparationcontract.PurposeResume,
		OperationID: input.OperationID, LifecycleID: record.ID, Generation: lease.Generation,
		Stage: stage, StartedAt: input.StartedAt, InvocationState: preparationcontract.InvocationNotInvoked,
		Workspace: input.Workspace,
		Probe:     preparationcontract.ProbeRequest{Repo: record.Repo, Host: binding.OwnerHost, Model: binding.OwnerModel, Effort: binding.OwnerEffort},
		Prepared: &preparationcontract.OrcaWorkspaceReceipt{
			Workspace: preparationcontract.WorkspaceReceipt{
				SourceRoot: execution.Workspace.SourceRoot, Root: execution.Workspace.Root,
				Branch: execution.Workspace.Branch, BaseHead: execution.Workspace.BaseHead,
				ParentWorktree: execution.Workspace.ParentWorktree, Driver: "orca", Exists: true,
			},
			RuntimeID: input.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
			WorktreeInstanceID: binding.WorktreeInstanceID,
		},
		Launch: &preparationcontract.LaunchIdentity{
			PromptPath: input.Artifacts.OwnerPromptPath, PromptSHA256: input.Artifacts.OwnerPromptSHA256,
			ContextPacketPath: input.Artifacts.ContextPacketPath, ContextPacketSHA256: input.Artifacts.ContextPacketSHA256,
		},
		IssueBodySHA256: input.Artifacts.IssueBodySHA256, ClaimTokenSHA256: lease.ClaimTokenSHA256,
		TerminalPTYID: strings.TrimSpace(input.ReusedTerminalPTYID),
		PriorBinding: &preparationcontract.ResumeBinding{
			RuntimeID: binding.RuntimeID, RepoID: binding.RepoID, WorktreeID: binding.WorktreeID,
			WorktreeInstanceID: binding.WorktreeInstanceID, LeaseGeneration: binding.LeaseGeneration,
			OwnerHost: binding.OwnerHost, OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
			RunID: binding.RunID, TaskID: binding.TaskID, DispatchID: binding.DispatchID,
			TerminalPTYID: binding.TerminalPTYID,
		},
		ResumeLease: &lease,
	}
	return SealIntent(intent, issue)
}

func ApplyResumeIntent(record preparationcontract.Record, intent preparationcontract.Intent) preparationcontract.Record {
	execution := *record.Execution
	record.Execution = &execution
	record.Execution.Pending = &preparationcontract.ExternalIntent{
		OperationID: intent.OperationID, Kind: PendingKind(intent.Stage),
		Marker: intent.Marker, StartedAt: intent.StartedAt,
	}
	record.Execution.Failure = nil
	return record
}
