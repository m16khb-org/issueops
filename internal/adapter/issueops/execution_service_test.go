package issueops

import executionapp "issueops/internal/application/issueopsexecution"

func testExecutionService() executionapp.Service {
	return executionapp.Service{ReadRecord: ReadIssueOps, SamePath: samePath, Verifier: liveTestVerifier()}
}
