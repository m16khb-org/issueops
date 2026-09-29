package issueopsapp

import (
	"io"

	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/mcpcli/resources"
	"issueops/cmd/issueops/pathutil"
	"issueops/internal/adapter/docs"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/inspect"
	issueopsadapter "issueops/internal/adapter/issueops"
	provenanceadapter "issueops/internal/adapter/outbound/issueopsprovenance"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/policy"
	"issueops/internal/adapter/preflight"
	preflightapp "issueops/internal/application/preflight"
)

func runMCP() error {
	return mcpcli.RunMCPWithDependencies(issueOpsMCPDependencies())
}

func serveMCPStream(input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return mcpcli.ServeMCPStreamWithDependencies(input, output, diagnostics, issueOpsMCPDependencies())
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
	inspector := newHarnessInspector()
	compatibility := compatibilityContract()
	stateRoot := issueOpsStateRoot()
	return mcpcli.MCPDependencies{
		APIDoc:        newAPIDocService(),
		DefaultTarget: resolveTarget(""),
		Inspect:       func(repo string) any { return inspector(repo) },
		Preflight:     preflightapp.Service{Observer: preflight.GitObserver{}},
		Skills:        inspect.ListSkills,
		Compatibility: func() any { return compatibility },
		Commit:        newCommitService(resolveTarget("")),
		Lint:          newLintService(resolveTarget("")),
		Fetch:         newWebFetch(),
		Execution:     mcpcli.ExecutionDeps{ExecuteExecution: issueopsadapter.ExecuteExecution, ObserveNativeProcessAncestry: issueopsadapter.ObserveNativeProcessAncestry, IssueOpsStateRoot: func() string { return stateRoot }},

		Gates:   newGatesService(),
		Channel: newChannelService(statestore.StateDir()),
		Policy:  policyService, Audit: newCommandAuditService(policyService),
		Worker:           newWorkerService(),
		Daemon:           newDaemonReader(),
		Catalog:          mcpcatalog.Build(),
		Loop:             newLoopService(),
		ProjectDocs:      docsService,
		ProjectBootstrap: newProjectBootstrapService(resolveTarget("")),
		State:            mcpcli.StateDependencies{Write: state.Write, Read: state.Read, List: state.List, Prune: state.Prune, Doctor: state.Doctor, Maintain: state.Maintain},
		Resources: resources.Config{
			IssueOpsRoot: root, Version: version, SkillName: skillName,
			ReadHarnessFile: func(parts ...string) (string, error) { return pathutil.ReadHarnessFile(root, parts...) },
			StateList:       state.List, RouteProjectDocs: docsService.Route, DocsIndex: docs.DocsIndex, CommandPolicySummary: policy.CommandPolicySummary,
		},
		SelfHistory:  newSelfWorkflowHistory(statestore.StateDir()),
		SelfState:    newSelfWorkflowState(statestore.StateDir()),
		SelfPlanning: newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version),
		SelfVerify:   newSelfWorkflowExecutor(issueOpsRoot()),
		Prepare:      execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner, ReadIssue: execution.ReadIssue,
		Replace: newIssueOpsReplacementHandler(),
		Claim:   issueOpsClaimHandler, Release: issueOpsReleaseHandler, Reseed: issueOpsReseedHandler,
		Resume: issueOpsResumeHandler, Reconcile: issueOpsReconcileHandler, Complete: issueOpsCompleteHandler,
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: mcpcli.PublicationHandlers{
			Create: issueOpsPublicationCreateHandler, Reconcile: issueOpsPublicationReconcileHandler,
		},
	}
}
