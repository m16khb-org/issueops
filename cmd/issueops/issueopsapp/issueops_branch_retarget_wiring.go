package issueopsapp

import (
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

func newBranchRetargeter(root string, observe func(model.IssueOpsRemoteArtifactVerification) (string, error)) branchapp.Retargeter {
	return branchapp.Retargeter{Records: core.CycleRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), TargetBranch: observe, OriginPresent: core.OriginBranchPresent, Now: time.Now}
}
