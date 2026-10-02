package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli/executioncmd"
	issueopscore "issueops/internal/adapter/issueops"
	authorization "issueops/internal/adapter/outbound/issueopsauthorization"
	executionapp "issueops/internal/application/issueopsexecution"
)

// 실행 CLI는 액션 구현을 알지 않는다. 어댑터를 아는 곳은 composition root
// 하나뿐이다.
func newIssueOpsExecutionRunners() executioncmd.ExecutionDeps {
	return executioncmd.ExecutionDeps{
		ExecuteExecution:             newExecutionService().Execute,
		ObserveNativeProcessAncestry: issueopscore.ObserveNativeProcessAncestry,
		SwitchExecutionMode:          newModeSwitcher(),
		SyncExecutionBase:            issueopscore.VerifiedSyncExecutionBase(issueOpsActorVerifier()),
	}
}

func newExecutionService() executionapp.Service {
	return executionapp.Service{ReadRecord: issueopscore.ReadIssueOps, SamePath: (authorization.CanonicalPaths{}).Same, Verifier: issueOpsActorVerifier()}
}
