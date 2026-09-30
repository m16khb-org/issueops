package executioncmd

import (
	adapter "issueops/internal/adapter/issueops"
	authorization "issueops/internal/adapter/outbound/issueopsauthorization"
	executionapp "issueops/internal/application/issueopsexecution"
)

func testExecutionService() executionapp.Service {
	return executionapp.Service{ReadRecord: adapter.ReadIssueOps, SamePath: (authorization.CanonicalPaths{}).Same, InspectProcess: adapter.InspectNativeProcessReceipt}
}
