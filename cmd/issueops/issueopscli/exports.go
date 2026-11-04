package issueopscli

import (
	"context"
	"issueops/cmd/issueops/issueopscli/benchmarkcmd"
	"issueops/cmd/issueops/issueopscli/executioncmd"
	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	"issueops/cmd/issueops/issueopscli/remotecmd"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	basesyncport "issueops/internal/port/issueopsbasesync"
	provenanceport "issueops/internal/port/issueopsprovenance"
)

// RemoteVerification 타입은 원격 산출물 검증 함수 묶음이다. Child는 자식 이슈 URL,
// Verify·VerifyContext는 산출물 존재, Merged는 병합 여부를 확인한다.
type RemoteVerification struct {
	Child         func(string) error
	Verify        func(issueopscontract.RemoteArtifactVerificationRequest) error
	VerifyContext func(context.Context, issueopscontract.RemoteArtifactVerificationRequest) error
	Merged        func(context.Context, issueopscontract.RemoteArtifactVerification) error
}

// Dependencies 타입은 RunIssueOpsWithDependencies가 하위 명령에 연결하는 핸들러와
// 런타임 의존성을 한데 모은다. 조립은 composition root에서 한다.
type Dependencies struct {
	Verification   RemoteVerification
	Remote         remotecmd.Command
	Cleanup        feedbackcleanup.Command
	CleanupRuntime feedbackcleanup.Deps
	Benchmark      benchmarkcmd.Command
	Execution      executioncmd.ExecutionDeps
	Runtime        Deps
	Gates          LoopGateDeps
	Usage          string
	ChildUsage     string
	Prepare        issueopscontract.ExecutionPrepareHandler
	BaseSync       basesyncport.Inspector
	Claim          issueopscontract.ExecutionClaimHandler
	Release        issueopscontract.ExecutionReleaseHandler
	Status         port.ExecutionStatusHandler
	Replace        port.ExecutionReplaceHandler
	Reseed         issueopscontract.ExecutionReseedHandler
	Complete       issueopscontract.ExecutionCompleteHandler
	Publication    remotecmd.PublicationHandlers
	Provenance     provenanceport.Observer
	HandoffCmux    issueopscontract.ExecutionCmuxHandoffHandler
}

// RunIssueOpsWithDependencies 함수는 args의 IssueOps 하위 명령을 deps의 핸들러로
// 실행한다.
func RunIssueOpsWithDependencies(args []string, deps Dependencies) error {
	return (command{Runtime: deps.Runtime, Gates: deps.Gates, VerifyChild: deps.Verification.Child}).runIssueOpsWithDependencies(args, deps)
}

// command owns the runtime used by one CLI invocation.
type command struct {
	VerifyChild func(string) error
	Runtime     Deps
	Gates       LoopGateDeps
}
