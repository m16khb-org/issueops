package issueops

import (
	"issueops/internal/contract/issueops"
	"strings"
)

func OwnerPacket(record issueops.IssueOpsRecord, req issueops.ExecutionPrepareRequest, snapshot issueops.OwnerSnapshot, artifactManifest map[string]string, paths issueops.OwnerPaths, policy issueops.OwnerPolicyContext) issueops.OwnerContextPacket {
	commands := OwnerCommandsFor(record, req, snapshot.Issue.BodySHA256, paths, policy)
	packet := issueops.OwnerContextPacket{
		SchemaVersion: issueops.IssueOpsSchemaVersion, LifecycleID: record.ID, Mode: record.Execution.Mode,
		SourceRoot: record.Execution.Workspace.SourceRoot, WorktreeRoot: record.Execution.Workspace.Root,
		WorktreeBase: paths.WorktreeBase, Branch: record.Execution.Workspace.Branch,
		BaseHead: record.Execution.Workspace.BaseHead, CurrentHead: record.Execution.Workspace.BaseHead,
		LeaseGeneration: record.Execution.Lease.Generation, Issue: snapshot.Issue,
		OwnerHost: strings.ToLower(strings.TrimSpace(req.OwnerHost)), OwnerModel: strings.TrimSpace(req.OwnerModel), OwnerEffort: strings.TrimSpace(req.OwnerEffort),
		ReviewerModel: policy.ReviewerModel, ReviewerEffort: policy.ReviewerEffort,
		ResearchModel: policy.ResearchModel, ResearchEffort: policy.ResearchEffort,
		ReaderCheckModel: policy.ReaderCheckModel, ReaderCheckEffort: policy.ReaderCheckEffort,
		RequiredDocs: snapshot.RequiredDocs, RequiredSkills: snapshot.RequiredSkills, AcceptanceIDs: snapshot.AcceptanceIDs,
		Verification: snapshot.VerificationCommands, VerificationReportPath: paths.Report, Commands: commands,
		ArtifactManifest: artifactManifest,
	}
	return packet
}
