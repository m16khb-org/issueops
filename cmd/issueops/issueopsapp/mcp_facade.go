package issueopsapp

import (
	"fmt"
	"io"
	channeladapter "issueops/internal/adapter/channel"
	gatesadapter "issueops/internal/adapter/gates"
	"issueops/internal/adapter/inspect"
	"issueops/internal/adapter/looprun"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/preflight"
	"issueops/internal/adapter/projectdocs"

	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/selfworkflow"
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
	mcpcli.SelfVerify = func(request selfworkflow.SelfVerifyRequest) (selfworkflow.SelfAugmentResult, error) {
		result, err := selfVerify(request)
		if err != nil && isSelfVerificationGateError(err) {
			return result, fmt.Errorf("%w: %w", mcpcli.ErrSelfVerificationGateFailed, err)
		}
		return result, err
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
	return mcpcli.MCPDependencies{
		Catalog:     mcpcatalog.Build(),
		SelfHistory: newSelfWorkflowHistory(statestore.StateDir()),
		Prepare:     execution.Prepare, Orca: execution.Orca, OrcaOwner: execution.OrcaOwner, ReadIssue: execution.ReadIssue,
		Claim: issueOpsClaimHandler, Release: issueOpsReleaseHandler, Reseed: issueOpsReseedHandler,
		Resume: issueOpsResumeHandler, Reconcile: issueOpsReconcileHandler, Complete: issueOpsCompleteHandler,
		Provenance: provenanceadapter.NewExecutableObserver(),
		Publication: mcpcli.PublicationHandlers{
			Create: issueOpsPublicationCreateHandler, Reconcile: issueOpsPublicationReconcileHandler,
		},
	}
}
