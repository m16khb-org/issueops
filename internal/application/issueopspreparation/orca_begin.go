package issueopspreparation

import (
	"fmt"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

func ValidateOrcaBegin(begin OrcaBegin) error {
	return preparationdomain.ValidateSelectionReceipt(begin.Selection, begin.Command, begin.Probe, preparationcontract.ModeOrca)
}

func EnsureOrcaBeginUnprepared(record leasecontract.Record) error {
	if record.Execution != nil {
		return fmt.Errorf("IssueOps execution already exists; reconcile or inspect its current state")
	}
	return nil
}

func ApplyOrcaBegin(current leasecontract.Record, begin OrcaBegin, artifactDir string) (leasecontract.Record, preparationcontract.Intent, error) {
	if err := requireArtifactDir(artifactDir); err != nil {
		return current, preparationcontract.Intent{}, err
	}
	issue, err := preparationdomain.PrepareIssueIdentity(current.IssueURL, preparationcontract.DecodeIssueLinkEvidence(current.BranchPrepare))
	if err != nil {
		return current, preparationcontract.Intent{}, err
	}
	if begin.Owner.Provider != issue.Provider || begin.Owner.Issue != issue.Issue || begin.Probe.Provider != issue.Provider || begin.Probe.Issue != issue.Issue {
		return current, preparationcontract.Intent{}, fmt.Errorf("owner issue identity changed before Orca intent persistence")
	}
	if begin.Command.OwnerHost != begin.Probe.Host || begin.Command.OwnerModel != begin.Probe.Model || begin.Command.OwnerEffort != begin.Probe.Effort {
		return current, preparationcontract.Intent{}, fmt.Errorf("owner profile changed before Orca intent persistence")
	}
	intent := preparationcontract.Intent{
		SchemaVersion: leasecontract.SchemaVersion, Purpose: preparationcontract.PurposePrepare,
		OperationID: begin.OperationID, LifecycleID: current.ID, Generation: 1,
		Stage: preparationcontract.IntentStageWorktree, StartedAt: begin.StartedAt,
		InvocationState: preparationcontract.InvocationNotInvoked,
		Workspace:       begin.Workspace, Probe: begin.Probe, IssueBodySHA256: begin.Owner.BodySHA256,
	}
	intent, err = preparationdomain.SealIntent(intent, issue)
	if err != nil {
		return current, preparationcontract.Intent{}, err
	}
	record := current
	selection := begin.Selection
	record.Execution = &leasecontract.Execution{
		Mode: preparationcontract.ModeOrca, Selection: &selection,
		Workspace: leasecontract.Workspace{
			SourceRoot: begin.Workspace.SourceRoot, Root: begin.Workspace.Root,
			Branch: begin.Workspace.Branch, BaseHead: begin.Workspace.BaseHead,
			ParentWorktree: begin.Workspace.ParentWorktree, Driver: "orca", LinkedAt: begin.StartedAt,
			ArtifactDir: artifactDir,
		},
		Lease: leasecontract.Lease{Generation: 1, Status: "released"},
		Pending: &leasecontract.ExternalIntent{
			OperationID: begin.OperationID, Kind: preparationdomain.PendingKind(intent.Stage), Marker: intent.Marker, StartedAt: begin.StartedAt,
		},
	}
	return record, intent, nil
}
