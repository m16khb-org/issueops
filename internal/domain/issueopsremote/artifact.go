package remote

import (
	"fmt"
	"strings"
)

type ArtifactAuthority struct {
	Phase          string
	IssueURL       string
	CodeProjectKey string
}

type Artifact struct {
	Provider     string
	Kind         string
	URL          string
	Labels       []string
	Assignees    []string
	TargetBranch string
}

func ProjectArtifact(authority ArtifactAuthority, candidate Artifact) (Artifact, error) {
	if authority.Phase != "pr" {
		return Artifact{}, fmt.Errorf("cannot verify remote artifact before pr phase")
	}
	provider := strings.ToLower(strings.TrimSpace(candidate.Provider))
	if provider != "github" && provider != "gitlab" {
		return Artifact{}, fmt.Errorf("remote artifact provider must be github or gitlab")
	}
	if issueProvider := ProviderFromURL(authority.IssueURL); issueProvider != "" && provider != issueProvider {
		return Artifact{}, fmt.Errorf("remote artifact provider must match linked issue provider")
	}
	kind := strings.ToLower(strings.TrimSpace(candidate.Kind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	if kind != "pr" && kind != "mr" {
		return Artifact{}, fmt.Errorf("remote artifact kind must be pr or mr")
	}
	if provider == "github" && kind != "pr" {
		return Artifact{}, fmt.Errorf("github remote artifact kind must be pr")
	}
	if provider == "gitlab" && kind != "mr" {
		return Artifact{}, fmt.Errorf("gitlab remote artifact kind must be mr")
	}
	artifactURL := strings.TrimSpace(candidate.URL)
	if err := ValidateArtifactURL(artifactURL, provider, kind); err != nil {
		return Artifact{}, err
	}
	codeProjectKey := authority.CodeProjectKey
	expectedProject := EffectiveProjectKey(codeProjectKey, authority.IssueURL, provider)
	if err := ValidateArtifactMatchesProject(expectedProject, artifactURL, provider, kind); err != nil {
		return Artifact{}, err
	}
	labels := CleanValues(candidate.Labels)
	if len(labels) == 0 {
		return Artifact{}, fmt.Errorf("remote artifact labels are required")
	}
	assignees := CleanValues(candidate.Assignees)
	if len(assignees) == 0 {
		return Artifact{}, fmt.Errorf("remote artifact assignees are required")
	}
	if invalid := InvalidAssignee(assignees); invalid != "" {
		return Artifact{}, fmt.Errorf("remote artifact assignee must be a verified provider user, not placeholder %q", invalid)
	}
	return Artifact{
		Provider:     provider,
		Kind:         kind,
		URL:          artifactURL,
		Labels:       labels,
		Assignees:    assignees,
		TargetBranch: strings.TrimSpace(candidate.TargetBranch),
	}, nil
}
