package issueopsremote

import (
	"fmt"
	"strings"
)

// CleanupBranchArtifactProject returns the canonical project key of the remote artifact url for provider, accepting pull_request and merge_request as aliases of kind. It errors when the URL does not identify one project.
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

// ValidateCleanupBranchOrigin requires the git origin remote to belong to the same project (artifactKey) as the remote artifact, so branch cleanup targets the artifact's own project.
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
