package issueopsapp

import (
	"context"
	"time"

	"issueops/internal/adapter/gitworktree"
	preparationinbound "issueops/internal/adapter/inbound/issueopspreparation"
	preparationoutbound "issueops/internal/adapter/outbound/issueopspreparation"
	"issueops/internal/adapter/outbound/sqlstore"
	preflightadapter "issueops/internal/adapter/preflight"
	preparationapp "issueops/internal/application/issueopspreparation"
	issueopscontract "issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	"issueops/internal/port"
)

type issueOpsPreparationCompositionDeps struct {
	Direct        port.ExecutionWorkspaceProvisioner
	Now           func() time.Time
	ValidateActor func(context.Context, issueopscontract.NativeActor) error
}

type issueOpsExecutionCompositionDeps struct {
	Prepare issueopscontract.ExecutionPrepareHandler
}

func productionIssueOpsExecutionDependencies() issueOpsExecutionCompositionDeps {
	return issueOpsExecutionCompositionDeps{
		Prepare: newIssueOpsPreparationHandler(issueOpsPreparationCompositionDeps{
			Direct:        gitworktree.Provisioner{GitCmd: preflightadapter.GitCmd, GitOut: preflightadapter.GitOut},
			ValidateActor: verifyIssueOpsPreparationActor,
		}),
	}
}

func newIssueOpsPreparationHandler(deps issueOpsPreparationCompositionDeps) issueopscontract.ExecutionPrepareHandler {
	return func(ctx context.Context, stateRoot string, request issueopscontract.ExecutionPrepareRequest) (issueopscontract.ExecutionPrepareResult, error) {
		if deps.ValidateActor != nil {
			if err := deps.ValidateActor(ctx, request.Actor); err != nil {
				return issueopscontract.ExecutionPrepareResult{ID: request.ID}, err
			}
		}
		service, err := newIssueOpsPreparationService(stateRoot, deps)
		if err != nil {
			return issueopscontract.ExecutionPrepareResult{ID: request.ID}, err
		}
		return preparationinbound.NewHandler(service)(ctx, stateRoot, request)
	}
}

func newIssueOpsPreparationService(stateRoot string, deps issueOpsPreparationCompositionDeps) (*preparationapp.Service, error) {
	database, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	repository := preparationoutbound.NewSQLiteRepository(database)
	direct := preparationoutbound.NewDirectWorkspace(deps.Direct)
	ownerContext := newIssueOpsOwnerContext(stateRoot)
	evidence := preparationoutbound.NewEvidence(preparationoutbound.EvidenceDependencies{
		Workspace: ownerContext.ResolveWorkspace,
		MaterializeDirect: func(_ context.Context, snapshot preparationcontract.Snapshot, receipt preparationcontract.WorkspaceReceipt) error {
			return ownerContext.MaterializeDirect(snapshot, receipt)
		},
	})
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return preparationapp.NewService(repository, preparationClock{now}, direct, evidence), nil
}

type preparationClock struct{ now func() time.Time }

func (clock preparationClock) Now() time.Time { return clock.now() }

func verifyIssueOpsPreparationActor(ctx context.Context, actor issueopscontract.NativeActor) error {
	_, err := issueOpsActorVerifier().Verify(ctx, actor)
	return err
}
