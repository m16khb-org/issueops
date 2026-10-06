package projectdocs

import (
	"issueops/internal/adapter/projectdocs/detection"
	projectdoc "issueops/internal/domain/projectdoc"
)

func inferProjectTypes(root string, signals projectdoc.ProjectSignals, frameworks []string, monorepo bool, addEvidence func(string)) []string {
	return detection.ProjectTypes(root, signals.Languages, frameworks, monorepo, addEvidence)
}
