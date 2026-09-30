package projectdocs

import (
	"path"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

type RouteEffects interface {
	Path(root, rel string) string
	Exists(path string) bool
	Now() time.Time
}

func Route(root, task string, effects RouteEffects) projectdocscontract.ProjectDocsRouteResult {
	normalizedTask := projectdocdomain.NormalizeRouteTask(task)
	rels := projectdocdomain.RouteDocsForTask(normalizedTask)
	entries := make([]projectdocscontract.ProjectDocRouteEntry, 0, len(rels)*2)
	for _, doc := range rels {
		filePath := effects.Path(root, doc.Rel)
		entries = append(entries, projectdocscontract.ProjectDocRouteEntry{RelPath: doc.Rel, Path: filePath, Reason: doc.Reason, Exists: effects.Exists(filePath)})
		if family, ok := projectdocdomain.FamilyByRoot(path.Base(doc.Rel)); ok {
			overviewRel := path.Join(projectdocdomain.ProjectDocsDir, family.OverviewRel())
			overviewPath := effects.Path(root, overviewRel)
			if effects.Exists(overviewPath) {
				entries = append(entries, projectdocscontract.ProjectDocRouteEntry{
					RelPath: overviewRel, Path: overviewPath, Reason: "family module detail for " + family.Root, Exists: true,
				})
			}
		}
	}
	warnings := []string{}
	if !effects.Exists(effects.Path(root, projectdocdomain.ProjectDocsDir)) {
		warnings = append(warnings, "project docs are missing; run issueops project bootstrap to create AGENTS.md routing, .issueops docs, and repo metadata")
	}
	return projectdocscontract.ProjectDocsRouteResult{
		OK: true, Kind: "project_docs_route", RepoRoot: root, Task: normalizedTask,
		GeneratedAt: effects.Now().Format(time.RFC3339), Docs: entries, Warnings: warnings,
	}
}
