package issueopsapp

import (
	"context"
	"os"

	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/mcpcli/resources"
	"issueops/cmd/issueops/pathutil"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/inspect"
	issueopsadapter "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	basesyncoutbound "issueops/internal/adapter/outbound/issueopsbasesync"
	provenanceadapter "issueops/internal/adapter/outbound/issueopsprovenance"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/policy"
	"issueops/internal/adapter/preflight"
	preflightapp "issueops/internal/application/preflight"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	projectdocscontract "issueops/internal/contract/projectdocs"
)

func runMCP() error {
	return mcpcli.RunMCPWithDependencies(issueOpsMCPDependencies())
}

func mcpTools() []map[string]any {
	return mcpcatalog.Build().Tools
}

func issueOpsMCPDependencies() mcpcli.MCPDependencies {
	execution := productionIssueOpsExecutionDependencies()
	state := stateDependencies()
	root := issueOpsRoot()
	docsService := newProjectDocsService(resolveTarget(""))
	policyService := newPolicyService()
	compatibility := compatibilityContract()
	stateRoot := issueOpsStateRoot()
	deps := mcpcli.MCPDependencies{
		APIDoc:        newAPIDocService(),
		DefaultTarget: resolveTarget(""),
		Inspect:       scopedHarnessInspector(resolveTarget("")),
		Preflight:     preflightapp.Service{Observer: preflight.GitObserver{}},
		Skills:        inspect.ListSkills,
		Compatibility: func() any { return compatibility },
		Commit:        newCommitService(resolveTarget("")),
		Lint:          newLintService(resolveTarget("")),
		Fetch:         newWebFetch(),
		Execution:     mcpcli.ExecutionDeps{ExecuteExecution: newExecutionService().Execute, ObserveNativeProcessAncestry: issueopsadapter.ObserveNativeProcessAncestry, IssueOpsStateRoot: func() string { return stateRoot }},

		Gates:   newGatesService(),
		Channel: newChannelService(statestore.StateDir()),
		Policy:  policyService, Audit: newCommandAuditService(policyService),
		Worker:           newWorkerService(),
		Catalog:          mcpcatalog.Build(),
		Loop:             newLoopService(),
		ProjectDocs:      docsService,
		ProjectBootstrap: newProjectBootstrapService(resolveTarget("")),
		State:            mcpcli.StateDependencies{Write: state.Write, Read: state.Read, List: state.List, Prune: state.Prune, Doctor: state.Doctor, Maintain: state.Maintain},
		Resources: resources.Config{
			IssueOpsRoot: root, Version: version, SkillName: skillName,
			ReadHarnessFile: func(parts ...string) (string, error) { return pathutil.ReadHarnessFile(root, parts...) },
			StateList:       state.List, RouteProjectDocs: docsService.Route, DocsIndex: newDocsService().Index, CommandPolicySummary: policy.CommandPolicySummary,
		},
		SelfHistory:  newSelfWorkflowHistory(statestore.StateDir()),
		SelfState:    newSelfWorkflowState(statestore.StateDir()),
		SelfPlanning: newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version),
		SelfVerify:   newSelfWorkflowExecutor(issueOpsRoot()),
		Prepare:      execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner, ReadIssue: execution.ReadIssue,
		BaseSync: basesyncoutbound.NewInspector(basesyncoutbound.RunGit),
		Status:   issueOpsExecutionStatusHandler, Replace: newIssueOpsReplacementHandler(),
		Claim: issueOpsClaimHandler, Release: issueOpsReleaseHandler, Reseed: issueOpsReseedHandler,
		Resume: issueOpsResumeHandler, Reconcile: issueOpsReconcileHandler, Complete: issueOpsCompleteHandler,
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: mcpcli.PublicationHandlers{
			Create: issueOpsPublicationCreateHandler, Reconcile: issueOpsPublicationReconcileHandler,
		},
		RequestScope:  mcpcli.NewRequestScope(authorityoutbound.ScopeResolver{}, issueOpsMCPRecordRoots(stateRoot)),
		Credentials:   authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()},
		BindAuthority: bindIssueOpsAuthority,
		BindTrace:     issueOpsMCPTraceBinding(os.Stderr),
	}
	deps.ForRequest = issueOpsMCPRequestDependencies(deps)
	return deps
}

// issueOpsMCPHTTPDependencies roots every server-scoped default at the
// immutable install root: the shared service's cwd and environment never pick
// a workspace. Workspace tools get request-local dependencies from ForRequest.
func issueOpsMCPHTTPDependencies() mcpcli.MCPDependencies {
	deps := issueOpsMCPDependencies()
	root := issueOpsRoot()
	deps.DefaultTarget = root
	deps.Inspect = scopedHarnessInspector(root)
	docsService := newScopedProjectDocsService(root, root)
	deps.Resources.RouteProjectDocs = func(_ string, task string) (projectdocscontract.ProjectDocsRouteResult, error) {
		return docsService.Route(root, task)
	}
	deps.ForRequest = issueOpsMCPRequestDependencies(deps)
	return deps
}

// issueOpsMCPRequestDependencies rebuilds every cwd-capturing service from the
// verified request scope, so concurrent requests never share a workspace.
func issueOpsMCPRequestDependencies(base mcpcli.MCPDependencies) func(context.Context, authoritycontract.Scope, model.VerifiedActor) (context.Context, mcpcli.MCPDependencies, error) {
	return func(ctx context.Context, scope authoritycontract.Scope, _ model.VerifiedActor) (context.Context, mcpcli.MCPDependencies, error) {
		deps := base
		root, cwd := scope.WorkspaceRoot, scope.CWD
		deps.DefaultTarget = root
		deps.Inspect = scopedHarnessInspector(root)
		deps.Commit = newScopedCommitService(root, cwd)
		deps.Lint = newScopedLintService(root, cwd)
		deps.ProjectDocs = newScopedProjectDocsService(root, cwd)
		deps.ProjectBootstrap = newScopedProjectBootstrapService(root, cwd)
		deps.Loop = newScopedLoopService(cwd)
		deps.Gates = newScopedGatesService(root, cwd)
		deps.Worker = newScopedWorkerService()
		return ctx, deps, nil
	}
}

func issueOpsMCPRecordRoots(stateRoot string) mcpcli.RecordRoots {
	return func(_ context.Context, kind mcpcli.RecordKind, id string) ([]string, error) {
		switch kind {
		case mcpcli.RecordIssueOps:
			record, err := issueopsadapter.ReadIssueOps(stateRoot, id)
			if err != nil {
				return nil, err
			}
			roots := []string{record.Repo, record.WorktreePath}
			if record.Execution != nil {
				roots = append(roots, record.Execution.Workspace.SourceRoot, record.Execution.Workspace.Root)
			}
			return roots, nil
		case mcpcli.RecordLoop:
			loop, err := newLoopStore().ReadExisting(id)
			if err != nil {
				return nil, err
			}
			return []string{loop.Repo}, nil
		default:
			return nil, nil
		}
	}
}
