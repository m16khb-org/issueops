package remoteverification

import (
	"fmt"
	model "issueops/internal/contract/remoteverification"
	policydomain "issueops/internal/domain/policy"
	"strings"
)

func ArtifactTarget(provider, kind, url string, mergeOnly bool) (model.Target, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	switch provider + ":" + kind {
	case "github:pr", "gitlab:mr":
		return model.Target{Provider: provider, Kind: kind, URL: strings.TrimSpace(url)}, nil
	case "github:issue", "gitlab:issue":
		if !mergeOnly {
			return model.Target{Provider: provider, Kind: kind, URL: strings.TrimSpace(url)}, nil
		}
	}
	if mergeOnly {
		return model.Target{}, fmt.Errorf("unsupported remote artifact for merge verification: %s:%s", provider, kind)
	}
	return model.Target{}, fmt.Errorf("unsupported remote artifact verification: %s:%s", provider, kind)
}
func ValidateArtifact(url string, labels, assignees []string, artifact model.Artifact) error {
	if strings.TrimSuffix(strings.TrimSpace(artifact.URL), "/") != strings.TrimSuffix(strings.TrimSpace(url), "/") {
		return fmt.Errorf(
			"verified remote artifact URL %q does not match requested URL",
			policydomain.BoundedDiagnostic(artifact.URL, 2048),
		)
	}
	if err := requireRemoteValues("label", labels, artifact.Labels); err != nil {
		return err
	}
	if err := requireRemoteValues("assignee", assignees, artifact.Assignees); err != nil {
		return err
	}
	return nil
}

func RequireMerged(url string, merged bool) error {
	if !merged {
		return fmt.Errorf("remote artifact is not verified merged: %s", url)
	}
	return nil
}
func requireRemoteValues(kind string, required []string, actual []string) error {
	actualSet := map[string]bool{}
	for _, value := range actual {
		value = strings.TrimSpace(strings.ToLower(value))
		if value != "" {
			actualSet[value] = true
		}
	}
	missing := []string{}
	for _, value := range required {
		cleaned := strings.TrimSpace(strings.ToLower(value))
		if cleaned != "" && !actualSet[cleaned] {
			missing = append(missing, value)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("remote artifact missing verified %s(s): %s", kind, strings.Join(missing, ", "))
	}
	return nil
}
