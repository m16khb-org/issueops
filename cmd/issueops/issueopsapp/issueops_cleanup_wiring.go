package issueopsapp

import (
	"context"
	"os"
	"time"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	issueopscore "issueops/internal/adapter/issueops"
	orcaadapter "issueops/internal/adapter/orca"
	cleanupapp "issueops/internal/application/issueopscleanup"
	completionapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// cleanup CLI는 정리 구현과 의존 조립을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func configureIssueOpsCleanup() {
	reflectAudit := func(ctx context.Context, root string, record issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string, prov port.IssueProvider) error {
		return (cleanupapp.AuditReflector{Receipts: completionapp.NewCompletionReceipts(issueopscore.RemoteRecordStore{StateRoot: root}, time.Now)}).Reflect(ctx, record, completion, audit, prov)
	}
	finish := func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
		return issueopscore.CleanupFinish(ctx, stateRoot, req, issueopscore.CleanupFinishDeps{
			Git:                d.CleanupFinishGit,
			Processes:          issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses},
			OrcaTerminals:      orcaadapter.New(),
			ObserveArtifact:    issueopscore.ObserveRemoteArtifact,
			RemoveOrcaWorktree: d.RemoveOrcaWorktree,
			ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
				return reflectAudit(ctx, stateRoot, rec, completion, audit, prov)
			},
		})
	}
	feedbackcleanup.ConfigureCleanup(feedbackcleanup.CleanupDeps{
		Status: func(ctx context.Context, root, id string, merged bool, d feedbackcleanup.Deps) (issueopscontract.IssueOpsCleanupStatus, error) {
			service := cleanupapp.StatusService{
				Records:    issueopscore.CycleRecordStore{StateRoot: root},
				Structural: cleanupapp.StructuralStatus{Environment: issueopscore.CleanupStatusEnvironment{RunGit: issueopscore.GitCmd, ReadGit: issueopscore.GitOut}},
				Provider:   d.Provider, VerifyMergedHead: d.VerifyMergedHead, ReadIssueSnapshot: issueopscore.ReadRemoteIssueSnapshot, CurrentDirectory: os.Getwd,
				PreviewFinish: func(ctx context.Context, req issueopscontract.CleanupFinishRequest, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
					return finish(ctx, root, req, d, prov)
				},
			}
			return service.Status(ctx, id, merged)
		},
		AddIssueOpsFeedbackWithActor: issueopscore.AddIssueOpsFeedbackWithActor,
		CleanupAbandon: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupAbandonResult, error) {
			return issueopscore.CleanupAbandon(ctx, stateRoot, req, issueopscore.CleanupAbandonDeps{
				Orca: d.OrcaIntent, OrcaOwner: d.OrcaOwner,
				Processes:     issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses},
				OrcaTerminals: orcaadapter.New(),
				Remote:        prov,
			})
		},
		CleanupFinish: finish,
		CleanupRemoteBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
			return issueopscore.CleanupRemoteBranch(ctx, stateRoot, req, issueopscore.CleanupRemoteBranchDeps{
				VerifyMergedArtifact: d.VerifyMergedHead,
				ObserveArtifact:      issueopscore.ObserveRemoteArtifact,
				ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
					return reflectAudit(ctx, stateRoot, rec, completion, audit, prov)
				},
			})
		},
		CleanupLinkedBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
			return issueopscore.CleanupLinkedBranch(ctx, stateRoot, req, issueopscore.CleanupLinkedBranchDeps{
				ObserveLinkedBranches: issueopscore.ObserveGitHubLinkedBranches(issueopscore.LiveProviderCLI),
				DeleteLinkedBranch:    issueopscore.DeleteGitHubLinkedBranch(issueopscore.LiveProviderCLI),
			})
		},
		CloseIssueOpsChildren: func(root, id string, req issueopscontract.IssueOpsCloseChildrenRequest, d feedbackcleanup.Deps) (issueopscontract.IssueOpsCloseChildrenResult, error) {
			return (cleanupapp.ChildrenCloser{Records: issueopscore.CycleRecordStore{StateRoot: root}, Provider: d.Provider, VerifyMerged: d.VerifyMerged, Now: time.Now}).Close(context.Background(), id, req.MergeEvidenceRequested, req.Confirm)
		},
		IssueOpsStateRoot: issueopscore.IssueOpsStateRoot,
		MarkIssueOpsContractFeedbackIssueUpdatedWithActor: issueopscore.MarkIssueOpsContractFeedbackIssueUpdatedWithActor,
		ObserveNativeProcessAncestry:                      issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                                      issueopscore.ReadIssueOps,
		ReadRemoteIssueSnapshot:                           issueopscore.ReadRemoteIssueSnapshot,
		ResolveRecordProvider:                             issuedomain.ResolveRecordProvider,
	})
}
