package issueops

import (
	"fmt"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
)

const (
	// AwaitBranchLinkInterval은 재관측 간격이다. provider readback은 네트워크
	// 호출이므로 촘촘히 돌면 rate limit에 닿는다.
	AwaitBranchLinkInterval = 15 * time.Second
	// awaitBranchLinkDefaultTimeout은 coordinator가 createLinkedBranch를
	// 수행하는 데 걸릴 만한 시간이다. 무한 대기는 owner를 영원히 붙잡는다.
	awaitBranchLinkDefaultTimeout = 10 * time.Minute
	// awaitBranchLinkMaxTimeout은 요청할 수 있는 상한이다. 이보다 오래
	// 기다려야 한다면 그것은 대기 문제가 아니라 coordinator가 멈춘 것이다.
	awaitBranchLinkMaxTimeout = 30 * time.Minute
)

func AwaitBranchLinkGates(record issueopscontract.IssueOpsRecord, observationAvailable bool) []string {
	var missing []string
	if !observationAvailable {
		missing = append(missing, "linked_branch_observation_unavailable")
	}
	prepare := record.BranchPrepare
	if prepare == nil {
		return append(missing, "branch_prepare_missing")
	}
	if prepare.Provider != "github" {
		// GitLab은 prepare 시점에 link_verified를 이미 요구하므로 이 창이 없다.
		missing = append(missing, "branch_await_link_is_github_only")
	}
	if strings.TrimSpace(prepare.IssueURL) == "" {
		missing = append(missing, "issue_url_missing")
	}
	if strings.TrimSpace(prepare.Branch) == "" {
		missing = append(missing, "branch_missing")
	}
	if strings.TrimSpace(prepare.BaseSHA) == "" {
		missing = append(missing, "sealed_base_missing")
	}
	return missing
}

func AwaitBranchLinkTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return awaitBranchLinkDefaultTimeout, nil
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("branch await-link --timeout is not a duration: %q", raw)
	}
	if timeout <= 0 || timeout > awaitBranchLinkMaxTimeout {
		return 0, fmt.Errorf("branch await-link --timeout must be between 0 and %s, got %s", awaitBranchLinkMaxTimeout, timeout)
	}
	return timeout, nil
}
