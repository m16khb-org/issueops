package issueopspreparation

import (
	"fmt"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

type IntentIdentityPaths struct{ Source, Root, Parent, PreparedRecordedRoot, PreparedRoot, PreparedParent bool }

func ValidateIntentIssueIdentity(record preparationcontract.Record, intent preparationcontract.Intent) error {
	identity, err := PrepareIssueIdentity(record.IssueURL, preparationcontract.DecodeIssueLinkEvidence(record.BranchPrepare))
	if err != nil {
		return err
	}
	if record.ID != intent.LifecycleID || intent.Probe.Provider != identity.Provider || intent.Probe.Issue != identity.Issue {
		return contractError("intent_identity_mismatch", "Orca intent issue identity changed before persistence")
	}
	return nil
}

func ValidateIntentRecordIdentity(record preparationcontract.Record, payload preparationcontract.Intent, paths IntentIdentityPaths) error {
	if err := ValidateIntentIssueIdentity(record, payload); err != nil {
		return err
	}
	if record.ID != payload.LifecycleID || record.Execution == nil || record.Execution.Mode != "orca" ||
		!paths.Source || !paths.Root ||
		record.Execution.Workspace.Branch != payload.Workspace.Branch || record.Execution.Workspace.BaseHead != payload.Workspace.BaseHead ||
		!paths.Parent ||
		record.Execution.Workspace.Driver != "orca" {
		return fmt.Errorf("Orca intent record identity changed")
	}
	if payload.Prepared != nil && (!paths.PreparedRecordedRoot ||
		!paths.PreparedRoot || record.Execution.Workspace.Branch != payload.Prepared.Workspace.Branch ||
		record.Execution.Workspace.BaseHead != payload.Prepared.Workspace.BaseHead ||
		!paths.PreparedParent) {
		return fmt.Errorf("Orca prepared workspace identity changed")
	}
	return nil
}

func ValidateLaunchRequest(request preparationcontract.IntentRequest, intent preparationcontract.Intent) error {
	if request.Stage != intent.Stage || request.Marker != intent.Marker || request.Launch == nil || intent.Launch == nil ||
		request.Launch.PromptPath != intent.Launch.PromptPath || request.Launch.PromptSHA256 != intent.Launch.PromptSHA256 ||
		request.Launch.ContextPacketPath != intent.Launch.ContextPacketPath || request.Launch.ContextPacketSHA256 != intent.Launch.ContextPacketSHA256 {
		return fmt.Errorf("sealed owner launch identity changed")
	}
	return nil
}
