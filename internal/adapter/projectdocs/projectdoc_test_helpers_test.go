package projectdocs

import (
	"issueops/internal/adapter/projectdoc"
	docdomain "issueops/internal/domain/projectdoc"
)

func plannedFileAction(path, content string) string {
	return projectdoc.PlannedFileAction(path, content)
}
func normalizeProjectDocRelPath(rel string) (string, error) { return docdomain.NormalizeRelPath(rel) }
