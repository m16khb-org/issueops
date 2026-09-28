package issueopscli

import (
	"context"
	"os"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	issueopscore "issueops/internal/adapter/issueops"
	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// cleanup CLI는 정리 구현과 의존 조립을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func wireCleanupForTests() {
	finish := func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
		return issueopscore.CleanupFinish(ctx, stateRoot, req, issueopscore.CleanupFinishDeps{
			Git:                d.CleanupFinishGit,
			Processes:          issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses},
			RemoveOrcaWorktree: d.RemoveOrcaWorktree,
			ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
				return issueopscore.ReflectCleanupAudit(issueopscore.IssueOpsStateRoot(), rec, completion, audit, prov)
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
			return issueopscore.CleanupAbandon(ctx, stateRoot, req, issueopscore.CleanupAbandonDeps{Orca: d.OrcaIntent, OrcaOwner: d.OrcaOwner})
		},
		CleanupFinish: finish,
		CleanupRemoteBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
			return issueopscore.CleanupRemoteBranch(ctx, stateRoot, req, issueopscore.CleanupRemoteBranchDeps{
				VerifyMergedArtifact: d.VerifyMergedHead,
				ReflectAudit: func(rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
					return issueopscore.ReflectCleanupAudit(issueopscore.IssueOpsStateRoot(), rec, completion, audit, prov)
				},
			})
		},
		CloseIssueOpsChildren: issueopscore.CloseIssueOpsChildren,
		IssueOpsStateRoot:     issueopscore.IssueOpsStateRoot,
		MarkIssueOpsContractFeedbackIssueUpdatedWithActor: issueopscore.MarkIssueOpsContractFeedbackIssueUpdatedWithActor,
		ObserveNativeProcessAncestry:                      issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                                      issueopscore.ReadIssueOps,
		ReadRemoteIssueSnapshot:                           issueopscore.ReadRemoteIssueSnapshot,
		ReflectCleanupAudit:                               issueopscore.ReflectCleanupAudit,
		ResolveRecordProvider:                             issuedomain.ResolveRecordProvider,
	})
}
