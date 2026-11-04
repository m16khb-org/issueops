package projectdoc

import (
	"net/url"
	"sort"
	"strings"

	projectdoccontract "issueops/internal/contract/projectdoc"
)

// BuildProfile assembles the project profile from scanned signals, the VCS
// classification, and detected frameworks and project types. Evidence collects
// manifest files found in the scan plus observedEvidence, deduplicated; the
// framework, project type, and evidence lists are sorted.
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

// ClassifyVCS derives the VCS provider and hosting kind from the origin remote
// URL. An empty origin means a local git repository when hasGit is true and no
// VCS otherwise; known hosts map to managed hosting and other hosts to
// self-hosted, with the provider guessed from the host name.
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

// RemoteHost returns the lowercase host of a git remote given as a URL or as
// scp-style user@host:path, or "" when none can be found.
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
	if _, after, ok := strings.Cut(remote, "@"); ok {
		rest := after
		if before, _, ok := strings.Cut(rest, ":"); ok {
			return strings.ToLower(before)
		}
		if before, _, ok := strings.Cut(rest, "/"); ok {
			return strings.ToLower(before)
		}
	}
	return ""
}
