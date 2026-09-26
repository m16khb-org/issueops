package projectdoc

import "testing"

func TestBuildProfileCollectsObservedAndDeclaredEvidence(t *testing.T) {
	signals := ProjectSignals{Files: []string{"go.mod", "cmd/issueops/main.go", "pnpm-workspace.yaml"}, Languages: []string{"Go"}, PackageManagers: []string{"pnpm"}}
	vcs := ClassifyVCS("git@github.com:org/repo.git", true)
	profile := BuildProfile(signals, vcs, []string{"Cobra"}, true, []string{"backend", "cli"}, []string{"go.mod:github.com/spf13/cobra", "cmd/"})
	if profile.VCS.Provider != "github" || !profile.Monorepo || len(profile.Frameworks) != 1 || len(profile.ProjectTypes) != 2 ||
		!containsProfileEvidence(profile.Evidence, "git remote/config") || !containsProfileEvidence(profile.Evidence, "go.mod") || !containsProfileEvidence(profile.Evidence, "go.mod:github.com/spf13/cobra") {
		t.Fatalf("profile=%+v", profile)
	}
}

func containsProfileEvidence(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestClassifyVCSFromObservedOrigin(t *testing.T) {
	for _, tc := range []struct {
		name, origin, provider, hosting, host string
		hasGit                                bool
	}{
		{name: "no git", provider: "none", hosting: "local"},
		{name: "local git", hasGit: true, provider: "git", hosting: "local"},
		{name: "github ssh", origin: "git@github.com:org/repo.git", provider: "github", hosting: "managed", host: "github.com"},
		{name: "self hosted gitlab", origin: "ssh://git@gitlab.internal/org/repo", provider: "gitlab", hosting: "self-hosted", host: "gitlab.internal"},
		{name: "unknown host", origin: "opaque", provider: "git", hosting: "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyVCS(tc.origin, tc.hasGit)
			if got.Provider != tc.provider || got.Hosting != tc.hosting || got.RemoteHost != tc.host {
				t.Fatalf("profile=%+v", got)
			}
		})
	}
}
