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
	if authority.Phase == "done" {
		// QA: agents re-ran verify-artifact on completed cycles to refresh
		// labels/assignees; the record keeps publication-time values and the
		// remote commands' readback is the live state.
		return Artifact{}, fmt.Errorf("the cycle is done; its PR/MR was verified at publication (status --field remote_artifact). For current labels, assignees, reviewers or title, read the PR/MR with gh/glab; remote label/assign/retitle/milestone read them back themselves")
	}
	if authority.Phase != "pr" {
		return Artifact{}, fmt.Errorf("cannot verify remote artifact before pr phase (current phase %q); verify-artifact follows remote create-pr", authority.Phase)
	}
	provider := strings.ToLower(strings.TrimSpace(candidate.Provider))
	if provider != "github" && provider != "gitlab" {
		return Artifact{}, fmt.Errorf("remote artifact provider must be github or gitlab")
	}
	if issueProvider := ProviderFromURL(authority.IssueURL); issueProvider != "" && provider != issueProvider {
		return Artifact{}, fmt.Errorf("remote artifact provider must match linked issue provider")
	}
	kind, err := normalizeArtifactKind(provider, candidate.Kind)
	if err != nil {
		return Artifact{}, err
	}
	artifactURL := strings.TrimSpace(candidate.URL)
	if err := ValidateArtifactURL(artifactURL, provider, kind); err != nil {
		return Artifact{}, err
	}
	expectedProject := EffectiveProjectKey(authority.CodeProjectKey, authority.IssueURL, provider)
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

// normalizeArtifactKind는 kind 별칭을 pr/mr로 정규화하고 provider와 맞는지 검증한다.
func normalizeArtifactKind(provider, rawKind string) (string, error) {
	kind := strings.ToLower(strings.TrimSpace(rawKind))
	switch kind {
	case "pull_request":
		kind = "pr"
	case "merge_request":
		kind = "mr"
	}
	if kind != "pr" && kind != "mr" {
		return "", fmt.Errorf("remote artifact kind must be pr or mr")
	}
	if provider == "github" && kind != "pr" {
		return "", fmt.Errorf("github remote artifact kind must be pr")
	}
	if provider == "gitlab" && kind != "mr" {
		return "", fmt.Errorf("gitlab remote artifact kind must be mr")
	}
	return kind, nil
}
