package issueopsapp

import (
	"io"
	"issueops/cmd/issueops/mcpcli/resources"
	"issueops/cmd/issueops/pathutil"
	channeladapter "issueops/internal/adapter/channel"
	"issueops/internal/adapter/docs"
	gatesadapter "issueops/internal/adapter/gates"
	"issueops/internal/adapter/inspect"
	"issueops/internal/adapter/looprun"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/policy"
	"issueops/internal/adapter/preflight"
	"issueops/internal/adapter/projectdocs"

	"issueops/cmd/issueops/mcpcli"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	provenanceadapter "issueops/internal/adapter/outbound/issueopsprovenance"
)

func configureMCPCLI() {
	mcpcli.Version = version
	mcpcli.IssueOpsRoot = issueOpsRoot
	mcpcli.ResolveTarget = resolveTarget
	mcpcli.RouteProjectDocs = projectdocs.RouteProjectDocs
	mcpcli.ReadProjectDoc = projectdocs.ReadProjectDoc
	mcpcli.ReviseProjectDoc = projectdocs.ReviseProjectDoc
	mcpcli.AppendProjectDocsEntry = projectdocs.AppendProjectDocsEntry
	mcpcli.LoopStart = looprun.Start
	mcpcli.LoopRecordAttempt = looprun.RecordAttempt
	mcpcli.LoopStop = looprun.Stop
	mcpcli.LoopStatus = looprun.Status
	mcpcli.GatesCheck = gatesadapter.Check
	mcpcli.GatesInit = gatesadapter.Init
	mcpcli.GatesAbandon = gatesadapter.Abandon
	mcpcli.ChannelSend = channeladapter.Send
	mcpcli.ChannelRecv = channeladapter.Recv
	mcpcli.GitPreflight = preflight.GitPreflight
	mcpcli.ListSkills = inspect.ListSkills
	mcpcli.ReadHarnessFile = readHarnessFile
	mcpcli.InspectHarness = func(repo string) any {
		return inspectHarness(repo)
	}
	mcpcli.DaemonStatus = func() any {
		return daemonStatusForMCP()
	}
	mcpcli.CompatibilityContract = func() any {
		return compatibilityContract()
	}
}

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
	return mcpcli.MCPDependencies{
		Catalog: mcpcatalog.Build(),
		State:   mcpcli.StateDependencies{Write: state.Write, Read: state.Read, List: state.List, Prune: state.Prune, Doctor: state.Doctor, Maintain: state.Maintain},
		Resources: resources.Config{
			IssueOpsRoot: root, Version: version, SkillName: skillName,
			ReadHarnessFile: func(parts ...string) (string, error) { return pathutil.ReadHarnessFile(root, parts...) },
			StateList:       state.List, RouteProjectDocs: projectdocs.RouteProjectDocs, DocsIndex: docs.DocsIndex, CommandPolicySummary: policy.CommandPolicySummary,
		},
		SelfHistory:  newSelfWorkflowHistory(statestore.StateDir()),
		SelfState:    newSelfWorkflowState(statestore.StateDir()),
		SelfPlanning: newSelfWorkflowPlanning(issueOpsRoot(), statestore.StateDir(), version),
		SelfVerify:   newSelfWorkflowExecutor(issueOpsRoot()),
		Prepare:      execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner, ReadIssue: execution.ReadIssue,
		Claim: issueOpsClaimHandler, Release: issueOpsReleaseHandler, Reseed: issueOpsReseedHandler,
		Resume: issueOpsResumeHandler, Reconcile: issueOpsReconcileHandler, Complete: issueOpsCompleteHandler,
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: mcpcli.PublicationHandlers{
			Create: issueOpsPublicationCreateHandler, Reconcile: issueOpsPublicationReconcileHandler,
		},
	}
}
