package remote

import (
	"reflect"
	"strings"
	"testing"
)

func artifactFixture() (ArtifactAuthority, Artifact) {
	return ArtifactAuthority{Phase: "pr", IssueURL: "https://gitlab.example/planning/backlog/-/issues/42", CodeProjectKey: "gitlab.example/team/service"}, Artifact{
		Provider: " gitlab ", Kind: " merge_request ", URL: " https://gitlab.example/team/service/-/merge_requests/7 ",
		Labels: []string{" bug ", "bug", ""}, Assignees: []string{" maintainer "}, TargetBranch: " main ",
	}
}

func TestProjectArtifactCanonicalizesWithoutMutatingInput(t *testing.T) {
	authority, candidate := artifactFixture()
	got, err := ProjectArtifact(authority, candidate)
	want := Artifact{Provider: "gitlab", Kind: "mr", URL: "https://gitlab.example/team/service/-/merge_requests/7", Labels: []string{"bug"}, Assignees: []string{"maintainer"}, TargetBranch: "main"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("projection = %+v, %v; want %+v", got, err, want)
	}
	got.Labels[0], got.Assignees[0] = "changed", "changed"
	if candidate.Labels[0] != " bug " || candidate.Assignees[0] != " maintainer " {
		t.Fatal("projection aliases caller metadata")
	}
}

func TestProjectArtifactRejectsInvalidAuthorityAndMetadata(t *testing.T) {
	cases := []struct {
		name   string
		change func(*ArtifactAuthority, *Artifact)
		want   string
	}{
		{"phase before provider", func(a *ArtifactAuthority, c *Artifact) { a.Phase = "feedback"; c.Provider = "unknown" }, "before pr phase"},
		{"unsupported provider", func(_ *ArtifactAuthority, c *Artifact) { c.Provider = "unknown" }, "provider must be github or gitlab"},
		{"linked provider", func(_ *ArtifactAuthority, c *Artifact) { c.Provider = "github" }, "match linked issue provider"},
		{"unknown kind", func(_ *ArtifactAuthority, c *Artifact) { c.Kind = "issue" }, "kind must be pr or mr"},
		{"gitlab kind", func(_ *ArtifactAuthority, c *Artifact) { c.Kind = "pr" }, "gitlab remote artifact kind must be mr"},
		{"github kind", func(a *ArtifactAuthority, c *Artifact) {
			a.IssueURL = "https://github.com/team/service/issues/1"
			c.Provider = "github"
			c.Kind = "mr"
		}, "github remote artifact kind must be pr"},
		{"URL shape", func(_ *ArtifactAuthority, c *Artifact) {
			c.URL = "https://gitlab.example/team/service/-/merge_requests/abc"
		}, "GitLab merge request URL"},
		{"sealed project", func(_ *ArtifactAuthority, c *Artifact) {
			c.URL = "https://gitlab.example/team/other/-/merge_requests/7"
		}, "match linked issue project"},
		{"unsealed project", func(a *ArtifactAuthority, _ *Artifact) { a.CodeProjectKey = "" }, "match linked issue project"},
		{"labels before assignees", func(_ *ArtifactAuthority, c *Artifact) { c.Labels = nil; c.Assignees = nil }, "labels are required"},
		{"empty assignees", func(_ *ArtifactAuthority, c *Artifact) { c.Assignees = []string{" "} }, "assignees are required"},
		{"placeholder assignee", func(_ *ArtifactAuthority, c *Artifact) { c.Assignees = []string{"@me"} }, "not placeholder"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authority, candidate := artifactFixture()
			tc.change(&authority, &candidate)
			got, err := ProjectArtifact(authority, candidate)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("projection = %+v, %v; want %s", got, err, tc.want)
			}
			if !reflect.DeepEqual(got, Artifact{}) {
				t.Fatalf("rejected artifact has usable projection: %+v", got)
			}
		})
	}
}

func TestProjectArtifactUsesLinkedProjectWithoutSeal(t *testing.T) {
	got, err := ProjectArtifact(ArtifactAuthority{Phase: "pr", IssueURL: "https://github.com/team/service/issues/42"}, Artifact{
		Provider: "GITHUB", Kind: "pull_request", URL: "https://github.com/team/service/pull/7", Labels: []string{"bug"}, Assignees: []string{"maintainer"},
	})
	if err != nil || got.Provider != "github" || got.Kind != "pr" {
		t.Fatalf("same-project artifact = %+v, %v", got, err)
	}
}
