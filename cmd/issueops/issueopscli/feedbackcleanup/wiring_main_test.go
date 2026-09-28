package feedbackcleanup

import (
	"context"
	"os"
	"testing"
	"time"

	issueopscore "issueops/internal/adapter/issueops"
	cleanupapp "issueops/internal/application/issueopscleanup"
	completionapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	issuedomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// 프로덕션에서는 issueopsapp이 주입한다. cleanup CLI 테스트는 실제 정리 경로를
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {

	finish := func(ctx context.Context, stateRoot string, req issueopscontract.CleanupFinishRequest, d Deps, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
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
	ConfigureCleanup(CleanupDeps{
		Status: func(ctx context.Context, root, id string, merged bool, d Deps) (issueopscontract.IssueOpsCleanupStatus, error) {
			service := cleanupapp.StatusService{
				Records:    issueopscore.CycleRecordStore{StateRoot: root},
				Structural: cleanupapp.StructuralStatus{Environment: issueopscore.CleanupStatusEnvironment{RunGit: issueopscore.GitCmd, ReadGit: issueopscore.GitOut}},
				Provider:   d.Provider, CurrentDirectory: os.Getwd,
				PreviewFinish: func(ctx context.Context, req issueopscontract.CleanupFinishRequest, prov port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
					return cleanupDeps.CleanupFinish(ctx, root, req, d, prov)
				},
			}
			return service.Status(ctx, id, merged)
		},
		AddIssueOpsFeedbackWithActor: issueopscore.AddIssueOpsFeedbackWithActor,
		CleanupAbandon: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupAbandonRequest, d Deps) (issueopscontract.CleanupAbandonResult, error) {
			runtime := issueopscore.CleanupAbandonRuntime{StateRoot: stateRoot, Git: d.CleanupFinishGit, Processes: issueopscore.CleanupProcessDeps{Observe: d.InspectCleanupProcesses}}
			return (cleanupapp.AbandonExecutor{
				Records: issueopscore.CleanupRecordStore{StateRoot: stateRoot}, Acquire: (issueopscore.CleanupLifetimeLock{StateRoot: stateRoot}).Acquire,
				Provider: d.Provider, Observe: cleanupapp.ObserveAbandonArtifact,
				Plan: (cleanupapp.AbandonPreviewer{
					Environment: runtime, ReadChild: runtime.ReadChild, Workspace: runtime.Workspace,
					Orca:   cleanupapp.AbandonOrcaObserver{ReadIntent: runtime.ReadIntent, InspectionRequest: runtime.InspectionRequest, Orca: d.OrcaIntent, Owner: d.OrcaOwner},
					Remote: cleanupapp.AbandonRemoteObserver{RemoteRef: (issueopscore.LinkedBranchRemoteRef{RunGit: runtime.Command}).Observe},
				}).Plan,
				NewAttempt: issueopscore.NewCleanupAttempt, Stop: runtime.Stop, Directory: (issueopscore.CleanupFinishEnvironment{}).Directory, Git: runtime.Command, Now: time.Now,
			}).Run(ctx, req)
		},
		CleanupFinish: finish,
		CleanupRemoteBranch: func(ctx context.Context, stateRoot string, req issueopscontract.CleanupRemoteBranchRequest, d Deps, prov port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
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
		CloseIssueOpsChildren: func(root, id string, req issueopscontract.IssueOpsCloseChildrenRequest, d Deps) (issueopscontract.IssueOpsCloseChildrenResult, error) {
			return (cleanupapp.ChildrenCloser{Records: issueopscore.CycleRecordStore{StateRoot: root}, Provider: d.Provider, VerifyMerged: d.VerifyMerged, Now: time.Now}).Close(context.Background(), id, req.MergeEvidenceRequested, req.Confirm)
		},
		IssueOpsStateRoot: issueopscore.IssueOpsStateRoot,
		MarkIssueOpsContractFeedbackIssueUpdatedWithActor: issueopscore.MarkIssueOpsContractFeedbackIssueUpdatedWithActor,
		ObserveNativeProcessAncestry:                      issueopscore.ObserveNativeProcessAncestry,
		ReadIssueOps:                                      issueopscore.ReadIssueOps,
		ReadRemoteIssueSnapshot:                           issueopscore.ReadRemoteIssueSnapshot,
		ResolveRecordProvider:                             issuedomain.ResolveRecordProvider,
	})
	os.Exit(m.Run())
}
