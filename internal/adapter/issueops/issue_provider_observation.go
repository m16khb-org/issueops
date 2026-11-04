package issueops

import (
	"context"
	"fmt"
	"issueops/internal/domain/issueopsremote"
	"strings"
)

// inferProviderFromRepoRemotes는 저장소의 remote URL을 관측해 provider를 단일
// 판별한다. record가 provider를 모를 때만 쓰이며, 최초 이슈 생성의 bootstrap
// 순환을 끊는 경로다(#300).
//
// 관측은 여기(adapter)가, 판별 규칙은 domain이 소유한다.
func inferProviderFromRepoRemotes(repo string) (string, error) {
	code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "remote", "-v")
	if code != 0 {
		return "", fmt.Errorf("cannot determine provider: repository remotes could not be observed; pass --provider github|gitlab")
	}
	urls := []string{}
	for line := range strings.SplitSeq(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			urls = append(urls, fields[1])
		}
	}
	return issueopsremote.ProviderFromRemoteURLs(urls)
}

func resolveProviderProjectAuthority(repo, provider string) (string, error) {
	code, out := defaultExecutionSyncBaseGit(context.Background(), repo, "remote", "get-url", "origin")
	if code != 0 {
		return "", fmt.Errorf("cannot determine project authority from origin remote")
	}
	projectKey, err := issueopsremote.ProjectKeyFromGitRemoteURL(strings.TrimSpace(out), provider)
	if err != nil {
		return "", fmt.Errorf("cannot determine project authority: %w", err)
	}
	return projectKey, nil
}
