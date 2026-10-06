package projectdocs

import (
	detection "issueops/internal/adapter/projectdocs/detection"
	projectdoccontract "issueops/internal/contract/projectdoc"
	projectdoc "issueops/internal/domain/projectdoc"
	"os"
	"path/filepath"
	"strings"
)

func inferProjectProfile(root string, signals projectdoc.ProjectSignals) projectdoccontract.ProjectProfile {
	vcs := inferProjectVCS(root)
	observedEvidence := []string{}
	addEvidence := func(value string) { observedEvidence = projectdoc.AppendUnique(observedEvidence, value) }
	frameworks := detection.Frameworks(root, signals.Files, addEvidence)
	monorepo := detection.Monorepo(root, signals.Files, addEvidence)
	projectTypes := inferProjectTypes(root, signals, frameworks, monorepo, addEvidence)
	return projectdoc.BuildProfile(signals, vcs, frameworks, monorepo, projectTypes, observedEvidence)
}

func inferProjectVCS(root string) projectdoccontract.ProjectVCSProfile {
	origin := ReadGitOriginURL(root)
	_, err := os.Stat(filepath.Join(root, ".git"))
	return projectdoc.ClassifyVCS(origin, err == nil)
}

func ReadGitOriginURL(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".git", "config"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	inOrigin := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inOrigin = trimmed == `[remote "origin"]`
			continue
		}
		if inOrigin && strings.HasPrefix(trimmed, "url") {
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}
