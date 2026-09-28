package issueops

import (
	"strings"

	"issueops/internal/contract/issueops"
)

// ResolveRecordProvider infers the VCS provider for a lifecycle record:
// branch-prepare/remote-artifact evidence first, then the issue URL host.
// URL 추론은 부분 문자열 매칭이라 "gitlab"이 들어간 self-hosted 도메인은
// gitlab으로 해석되고, 그 문자열이 없는 커스텀 도메인만 ""로 떨어져
// 명시 --provider가 필요하다.
func ResolveRecordProvider(record issueops.IssueOpsRecord) string {
	if record.BranchPrepare != nil && record.BranchPrepare.Provider != "" {
		return record.BranchPrepare.Provider
	}
	if record.RemoteArtifact != nil && record.RemoteArtifact.Provider != "" {
		return record.RemoteArtifact.Provider
	}
	if strings.Contains(record.IssueURL, "github.com") {
		return "github"
	}
	if strings.Contains(record.IssueURL, "gitlab") {
		return "gitlab"
	}
	return ""
}
