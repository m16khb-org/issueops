package remote

import (
	"fmt"
	"strings"
)

func CleanupBranchArtifactProject(url, provider, kind string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	default:
		kind = strings.ToLower(strings.TrimSpace(kind))
	}
	key := ProjectKey(strings.TrimSpace(url), provider, kind)
	if key == "" {
		return "", fmt.Errorf("remote artifact URL does not identify one %s project", provider)
	}
	return key, nil
}

func ValidateCleanupBranchOrigin(artifactKey, origin, provider string) error {
	originKey, err := ProjectKeyFromGitRemoteURL(strings.TrimSpace(origin), strings.ToLower(strings.TrimSpace(provider)))
	if err != nil {
		return err
	}
	if originKey != artifactKey {
		return fmt.Errorf("origin project %q does not match the remote artifact project %q", originKey, artifactKey)
	}
	return nil
}
