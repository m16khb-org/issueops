package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli"
	issueopscontract "issueops/internal/contract/issueops"
	policy "issueops/internal/contract/policy"
)

func verifyIssueOpsRemoteArtifactLive(req issueopscontract.IssueOpsRemoteArtifactVerificationRequest) error {
	return issueopscli.VerifyRemoteArtifactLive(req)
}

func parseCommandPolicyFlags(name string, args []string) (policy.CommandPolicyRequest, bool, error) {
	return newPolicyCommand().ParseFlags(name, args)
}

func parseCommandPolicyRunFlags(args []string) (policy.CommandPolicyRequest, bool, bool, error) {
	return newPolicyCommand().ParseRunFlags(args)
}
