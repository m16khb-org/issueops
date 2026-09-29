package issueopscli

import (
	"issueops/cmd/issueops/issueopscli/remotecmd"
	"issueops/cmd/issueops/issueopscli/remoteverify"
	executionissue "issueops/internal/contract/executionissue"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

type Dependencies struct {
	Runtime     IssueOpsCLIDeps
	Gates       LoopGateDeps
	Usage       string
	ChildUsage  string
	Prepare     issueopscontract.ExecutionPrepareHandler
	Orca        port.ExecutionOrcaProvisioner
	OrcaOwner   port.ExecutionOrcaOwnerInspector
	BaseSync    basesyncport.Inspector
	ReadIssue   executionissue.ExecutionIssueSnapshotReadFunc
	Claim       issueopscontract.ExecutionClaimHandler
	Release     issueopscontract.ExecutionReleaseHandler
	Reseed      issueopscontract.ExecutionReseedHandler
	Resume      issueopscontract.ExecutionResumeHandler
	Reconcile   port.ExecutionReconcileHandler
	Complete    issueopscontract.ExecutionCompleteHandler
	Publication remotecmd.PublicationHandlers
	Provenance  provenanceport.Observer
	HandoffCmux issueopscontract.ExecutionCmuxHandoffHandler
}

func RunIssueOpsWithDependencies(args []string, deps Dependencies) error {
	return (command{Runtime: deps.Runtime, Gates: deps.Gates}).runIssueOpsWithDependencies(args, deps)
}

func VerifyChildIssueBeforeLink(childURL string) error {
	return verifyIssueOpsChildIssueBeforeLink(childURL)
}

func VerifyRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return verifyIssueOpsRemoteArtifactLive(req)
}

func SetChildIssueVerifier(verifier func(string) error) func(string) error {
	return remoteverify.SetChildIssueVerifier(verifier)
}

// command owns the runtime used by one CLI invocation.
type command struct {
	Runtime IssueOpsCLIDeps
	Gates   LoopGateDeps
}
