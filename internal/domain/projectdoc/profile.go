package projectdoc

import (
	"net/url"
	"sort"
	"strings"

	projectdoccontract "issueops/internal/contract/projectdoc"
)

func BuildProfile(signals ProjectSignals, vcs projectdoccontract.ProjectVCSProfile, frameworks []string, monorepo bool, projectTypes, observedEvidence []string) projectdoccontract.ProjectProfile {
	profile := projectdoccontract.ProjectProfile{
		VCS: vcs, Languages: append([]string{}, signals.Languages...), PackageManagers: append([]string{}, signals.PackageManagers...),
		Frameworks: append([]string{}, frameworks...), Monorepo: monorepo, ProjectTypes: append([]string{}, projectTypes...), Evidence: []string{},
	}
	if vcs.RemoteHost != "" || vcs.Provider == "git" || vcs.Provider == "local" {
		profile.Evidence = AppendUnique(profile.Evidence, "git remote/config")
	}
	for _, rel := range signals.Files {
		switch {
		case rel == "go.mod" || strings.HasSuffix(rel, "/go.mod"),
			rel == "package.json" || strings.HasSuffix(rel, "/package.json"),
			rel == "pyproject.toml" || strings.HasSuffix(rel, "/pyproject.toml"),
			rel == "Cargo.toml" || strings.HasSuffix(rel, "/Cargo.toml"),
			rel == "pnpm-workspace.yaml", rel == "turbo.json", rel == "nx.json", rel == "lerna.json":
			profile.Evidence = AppendUnique(profile.Evidence, rel)
		}
	}
	for _, evidence := range observedEvidence {
		profile.Evidence = AppendUnique(profile.Evidence, evidence)
	}
	sort.Strings(profile.Frameworks)
	sort.Strings(profile.ProjectTypes)
	sort.Strings(profile.Evidence)
	return profile
}

func ClassifyVCS(origin string, hasGit bool) projectdoccontract.ProjectVCSProfile {
	if origin == "" {
		if hasGit {
			return projectdoccontract.ProjectVCSProfile{Provider: "git", Hosting: "local", RemoteName: "origin"}
		}
		return projectdoccontract.ProjectVCSProfile{Provider: "none", Hosting: "local"}
	}
	host := RemoteHost(origin)
	provider, hosting := "git", "self-hosted"
	switch strings.ToLower(host) {
	case "github.com":
		provider, hosting = "github", "managed"
	case "gitlab.com":
		provider, hosting = "gitlab", "managed"
	case "bitbucket.org":
		provider, hosting = "bitbucket", "managed"
	default:
		lowerHost := strings.ToLower(host)
		switch {
		case strings.Contains(lowerHost, "gitlab"):
			provider = "gitlab"
		case strings.Contains(lowerHost, "github"):
			provider = "github"
		case strings.Contains(lowerHost, "bitbucket"):
			provider = "bitbucket"
		}
	}
	if host == "" {
		hosting = "unknown"
	}
	return projectdoccontract.ProjectVCSProfile{Provider: provider, Hosting: hosting, RemoteHost: host, RemoteName: "origin"}
}

func RemoteHost(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.Contains(remote, "://") {
		u, err := url.Parse(remote)
		if err == nil {
			return strings.ToLower(u.Hostname())
		}
	}
	if at := strings.Index(remote, "@"); at >= 0 {
		rest := remote[at+1:]
		if colon := strings.Index(rest, ":"); colon >= 0 {
			return strings.ToLower(rest[:colon])
		}
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return strings.ToLower(rest[:slash])
		}
	}
	return ""
}
