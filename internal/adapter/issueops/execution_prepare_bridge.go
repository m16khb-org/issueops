package issueops

import (
	"encoding/json"
	"fmt"

	ownerapp "issueops/internal/application/issueopsowner"
	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

// ResolveExecutionPreparationWorkspace exposes the predecessor's canonical
// workspace calculation as a granular composition effect.
func ResolveExecutionPreparationWorkspace(snapshot preparationcontract.Snapshot, confirm bool) (preparationcontract.WorkspaceRequest, error) {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return preparationcontract.WorkspaceRequest{}, err
	}
	request, err := executionWorkspaceRequest(record, confirm)
	if err != nil {
		return preparationcontract.WorkspaceRequest{}, err
	}
	return preparationcontract.WorkspaceRequest{
		LifecycleID: request.LifecycleID, SourceRoot: request.SourceRoot, Root: request.Root,
		Branch: request.Branch, BaseBranch: request.BaseBranch, BaseHead: request.BaseHead,
		ParentWorktree: request.ParentWorktree, Confirm: request.Confirm,
	}, nil
}

func MaterializeExecutionPreparationDirect(stateRoot string, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
	record, err := executionPreparationCoreRecord(snapshot)
	if err != nil {
		return err
	}
	record.WorktreePath = receipt.Root
	record.Execution = &issueops.Execution{
		Mode: issueops.ExecutionModeDirect,
		Workspace: issueops.Workspace{
			SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch,
			BaseHead: receipt.BaseHead, ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver,
			ArtifactDir: ownerapp.OwnerArtifactDir(record),
		},
	}
	_, err = materializeStagedArtifacts(stateRoot, record)
	return err
}

func NewExecutionPreparationOperationID() (string, error) { return newExecutionOperationID() }

func executionPreparationCoreRecord(snapshot preparationcontract.Snapshot) (issueops.IssueOpsRecord, error) {
	if len(snapshot.RecordRaw) == 0 {
		return issueops.IssueOpsRecord{}, fmt.Errorf("preparation raw record snapshot is required")
	}
	var record issueops.IssueOpsRecord
	if err := json.Unmarshal(snapshot.RecordRaw, &record); err != nil {
		return issueops.IssueOpsRecord{}, err
	}
	return record, nil
}
