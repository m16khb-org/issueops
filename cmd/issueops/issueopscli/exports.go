package issueopscli

import (
	"context"
	"issueops/cmd/issueops/issueopscli/benchmarkcmd"
	"issueops/cmd/issueops/issueopscli/executioncmd"
	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	executionissue "issueops/internal/contract/executionissue"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

type RemoteVerification struct {
	Child         func(string) error
	Verify        func(issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error
	VerifyContext func(context.Context, issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error
	Merged        func(context.Context, issueopscontract.IssueOpsRemoteArtifactVerification) error
}

type Dependencies struct {
	Verification   RemoteVerification
	Remote         remotecmd.Command
	Cleanup        feedbackcleanup.Command
	CleanupRuntime feedbackcleanup.Deps
	Benchmark      benchmarkcmd.Command
	Execution      executioncmd.ExecutionDeps
	Runtime        IssueOpsCLIDeps
	Gates          LoopGateDeps
	Usage          string
	ChildUsage     string
	Prepare        issueopscontract.ExecutionPrepareHandler
	Orca           port.ExecutionOrcaProvisioner
	OrcaOwner      port.ExecutionOrcaOwnerInspector
	BaseSync       basesyncport.Inspector
	ReadIssue      executionissue.ExecutionIssueSnapshotReadFunc
	Claim          issueopscontract.ExecutionClaimHandler
	Release        issueopscontract.ExecutionReleaseHandler
	Status         port.ExecutionStatusHandler
	Replace        port.ExecutionReplaceHandler
	Reseed         issueopscontract.ExecutionReseedHandler
	Resume         issueopscontract.ExecutionResumeHandler
	Reconcile      port.ExecutionReconcileHandler
	Complete       issueopscontract.ExecutionCompleteHandler
	Publication    remotecmd.PublicationHandlers
	Provenance     provenanceport.Observer
	HandoffCmux    issueopscontract.ExecutionCmuxHandoffHandler
}

func RunIssueOpsWithDependencies(args []string, deps Dependencies) error {
	return (command{Runtime: deps.Runtime, Gates: deps.Gates, VerifyChild: deps.Verification.Child}).runIssueOpsWithDependencies(args, deps)
}

// command owns the runtime used by one CLI invocation.
type command struct {
	VerifyChild func(string) error
	Runtime     IssueOpsCLIDeps
	Gates       LoopGateDeps
}
