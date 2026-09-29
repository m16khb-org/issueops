package issueopscli

import (
	"context"
	reviewapp "issueops/internal/application/issueopsreview"
	"os"
	"time"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	issueopscore "issueops/internal/adapter/issueops"
	preparationoutbound "issueops/internal/adapter/outbound/issueopspreparation"
	cleanupapp "issueops/internal/application/issueopscleanup"
	preparationapp "issueops/internal/application/issueopspreparation"
	completionapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// cleanup CLI는 정리 구현과 의존 조립을 알지 않는다. 어댑터를 아는 곳은
// composition root 하나뿐이다.
func testCleanupCommand() feedbackcleanup.Command {

	finish := func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
		runtime := issueopscore.CleanupFinishRuntime{RunGit: d.CleanupFinishGit, Processes: issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses}}
		evidence := cleanupapp.FinishEvidenceReader{Provider: prov, VerifyMergedHead: d.VerifyMergedHead, ReadIssueSnapshot: issueopscore.ReadRemoteIssueSnapshot}
		return (cleanupapp.FinishExecutor{
			Records: issueopscore.CleanupRecordStore{StateRoot: stateRoot}, Acquire: (issueopscore.CleanupLifetimeLock{StateRoot: stateRoot}).Acquire,
			Observe: evidence.Observe,
			Plan: func(ctx context.Context, record issueopscontract.IssueOpsRecord, request issueopscontract.CleanupFinishRequest) (issueopscontract.CleanupFinishInventory, issueopscontract.CleanupFinishResult) {
				return (cleanupapp.FinishPreviewer{Environment: issueopscore.CleanupFinishEnvironment{RunGit: func(dir string, args ...string) (int, string) { return runtime.Git(ctx, dir, args...) }}, Workspace: runtime.Workspace}).Plan(ctx, record, request)
			},
			Fingerprint: issueopscore.CleanupFinishFingerprint, NewAttempt: issueopscore.NewCleanupAttempt,
			Completion: completionapp.NewCompletionCollector(issueopscore.CompletionArtifacts{}).Collect,
			Stop:       runtime.Stop, RemoveOrca: d.RemoveOrcaWorktree, Directory: (issueopscore.CleanupFinishEnvironment{}).Directory, Git: runtime.Git, Now: time.Now,
			ReflectAudit: func(ctx context.Context, rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
				return cleanupapp.WriteCleanupAudit(ctx, rec, completion, audit, prov)
			},
		}).Run(ctx, req)
	}
	return feedbackcleanup.Command{Operations: feedbackcleanup.CleanupDeps{
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
		AddIssueOpsFeedbackWithActor: func(root, id, source, body, classification string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.AddFeedback(issueopscore.NewReviewMutationStore(&actor), root, id, source, body, classification)
		},
		CleanupAbandon: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, d feedbackcleanup.Deps) (issueopscontract.CleanupAbandonResult, error) {
			runtime := issueopscore.CleanupAbandonRuntime{StateRoot: stateRoot, Git: d.CleanupFinishGit, Processes: issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses}}
			return (cleanupapp.AbandonExecutor{
				Records: issueopscore.CleanupRecordStore{StateRoot: stateRoot}, Acquire: (issueopscore.CleanupLifetimeLock{StateRoot: stateRoot}).Acquire,
				Provider: d.Provider, Observe: cleanupapp.ObserveAbandonArtifact,
				Plan: (cleanupapp.AbandonPreviewer{
					Environment: runtime, ReadChild: runtime.ReadChild, Workspace: runtime.Workspace,
					Orca: cleanupapp.AbandonOrcaObserver{ReadIntent: runtime.ReadIntent, InspectionRequest: func(record issueopscontract.IssueOpsRecord, intent preparationcontract.Intent) (port.ExecutionOrcaIntentRequest, error) {
						projected, err := issueopscore.PreparationIntentRecord(record)
						if err != nil {
							return port.ExecutionOrcaIntentRequest{}, err
						}
						request, err := (preparationapp.IntentRequestBuilder{Files: issueopscore.OrcaIntentFiles{}}).Inspect(projected, intent)
						return preparationoutbound.OrcaIntentRequest(request), err
					}, Orca: d.OrcaIntent, Owner: d.OrcaOwner},
					Remote: cleanupapp.AbandonRemoteObserver{RemoteRef: (issueopscore.LinkedBranchRemoteRef{RunGit: runtime.Command}).Observe},
				}).Plan,
				NewAttempt: issueopscore.NewCleanupAttempt, Stop: runtime.Stop, Directory: (issueopscore.CleanupFinishEnvironment{}).Directory, Git: runtime.Command, Now: time.Now,
			}).Run(ctx, req)
		},
		CleanupFinish: finish,
		CleanupRemoteBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, d feedbackcleanup.Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
			return (cleanupapp.RemoteBranchCleaner{
				Records: issueopscore.CleanupRecordStore{StateRoot: stateRoot},
				Acquire: (issueopscore.CleanupLifetimeLock{StateRoot: stateRoot}).Acquire, NewAttempt: issueopscore.NewCleanupAttempt,
				Preview:    cleanupapp.RemoteBranchPreviewer{Environment: issueopscore.CleanupRemoteBranchEnvironment{}, VerifyMergedArtifact: d.VerifyMergedHead},
				Completion: completionapp.NewCompletionCollector(issueopscore.CompletionArtifacts{}).Collect,
				Now:        time.Now,
				ReflectAudit: func(ctx context.Context, rec issueopscontract.IssueOpsRecord, completion issueopscontract.RemoteCompletionSection, audit string) error {
					return cleanupapp.WriteCleanupAudit(ctx, rec, completion, audit, prov)
				},
			}).Run(ctx, req)
		},
		CloseIssueOpsChildren: func(root, id string, req issueopscontract.IssueOpsCloseChildrenRequest, d feedbackcleanup.Deps) (issueopscontract.IssueOpsCloseChildrenResult, error) {
			return (cleanupapp.ChildrenCloser{Records: issueopscore.CycleRecordStore{StateRoot: root}, Provider: d.Provider, VerifyMerged: d.VerifyMerged, Now: time.Now}).Close(context.Background(), id, req.MergeEvidenceRequested, req.Confirm)
		},
		IssueOpsStateRoot: issueopscore.IssueOpsStateRoot,
		MarkIssueOpsContractFeedbackIssueUpdatedWithActor: func(root, id string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			return reviewapp.MarkContractFeedbackIssueUpdated(issueopscore.NewReviewMutationStore(&actor), root, id)
		},
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                 issueopscore.ReadIssueOps,
		ResolveRecordProvider:        issuedomain.ResolveRecordProvider,
	}}
}
