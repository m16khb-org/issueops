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

	finish := func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
		runtime := issueopscore.CleanupFinishRuntime{RunGit: d.CleanupFinishGit, Processes: issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses}, OrcaTerminals: orcaadapter.New()}
		evidence := cleanupapp.FinishEvidenceReader{Provider: prov, VerifyMergedHead: d.VerifyMergedHead, ReadIssueSnapshot: issueopscore.ReadRemoteIssueSnapshot}
		return (cleanupapp.FinishExecutor{
			Records: issueopscore.CleanupRecordStore{StateRoot: stateRoot}, Acquire: (issueopscore.CleanupLifetimeLock{StateRoot: stateRoot}).Acquire,
			Observe: evidence.Observe,
			Plan: func(ctx context.Context, record issueopscontract.IssueOpsRecord, request issueopscontract.CleanupFinishRequest) (issueopscontract.CleanupFinishInventory, issueopscontract.CleanupFinishResult) {
				return (cleanupapp.FinishPreviewer{Environment: issueopscore.CleanupFinishEnvironment{RunGit: func(dir string, args ...string) (int, string) { return runtime.Git(ctx, dir, args...) }}, ObserveArtifact: issueopscore.ObserveRemoteArtifact, Workspace: runtime.Workspace}).Plan(ctx, record, request)
			},
			Fingerprint: issueopscore.CleanupFinishFingerprint, NewAttempt: issueopscore.NewCleanupAttempt,
			Completion: completionapp.NewCompletionCollector(issueopscore.CompletionArtifacts{}).Collect,
			Stop:       runtime.Stop, RemoveOrca: d.RemoveOrcaWorktree, Directory: (issueopscore.CleanupFinishEnvironment{}).Directory, Git: runtime.Git, Now: time.Now,
			ReflectAudit: func(ctx context.Context, rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
				return cleanupapp.WriteCleanupAudit(ctx, rec, completion, audit, prov)
			},
		}).Run(ctx, req)
	}
	feedbackcleanup.ConfigureCleanup(feedbackcleanup.CleanupDeps{
		Status: func(ctx context.Context, root, id string, merged bool, d feedbackcleanup.Deps) (issueopscontract.IssueOpsCleanupStatus, error) {
			service := cleanupapp.StatusService{
				Records:    issueopscore.CycleRecordStore{StateRoot: root},
				Structural: cleanupapp.StructuralStatus{Environment: issueopscore.CleanupStatusEnvironment{RunGit: issueopscore.GitCmd, ReadGit: issueopscore.GitOut}},
				Provider:   d.Provider, CurrentDirectory: os.Getwd,
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
			return (cleanupapp.RemoteBranchCleaner{
				Records: issueopscore.CleanupRecordStore{StateRoot: stateRoot},
				Acquire: (issueopscore.CleanupLifetimeLock{StateRoot: stateRoot}).Acquire, NewAttempt: issueopscore.NewCleanupAttempt,
				Preview:    cleanupapp.RemoteBranchPreviewer{Environment: issueopscore.CleanupRemoteBranchEnvironment{}, VerifyMergedArtifact: d.VerifyMergedHead, ObserveArtifact: issueopscore.ObserveRemoteArtifact},
				Completion: completionapp.NewCompletionCollector(issueopscore.CompletionArtifacts{}).Collect,
				Now:        time.Now,
				ReflectAudit: func(ctx context.Context, rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
					return cleanupapp.WriteCleanupAudit(ctx, rec, completion, audit, prov)
				},
			}).Run(ctx, req)
		},
		CleanupLinkedBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
			return (cleanupapp.LinkedBranchCleaner{
				Records:               issueopscore.CycleRecordStore{StateRoot: stateRoot},
				RemoteRef:             issueopscore.LinkedBranchRemoteRef{}.Observe,
				Now:                   time.Now,
				ObserveLinkedBranches: issueopscore.ObserveGitHubLinkedBranches(issueopscore.LiveProviderCLI),
				DeleteLinkedBranch:    issueopscore.DeleteGitHubLinkedBranch(issueopscore.LiveProviderCLI),
			}).Run(ctx, req)
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
