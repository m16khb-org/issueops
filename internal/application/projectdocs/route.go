package projectdocs

import (
	"errors"
	"io/fs"
	"path"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

// maxRoutedRecords caps the dated records one route returns, so a broad task
// does not drown the routed documents.
const maxRoutedRecords = 5

type RouteEffects interface {
	Path(root, rel string) string
	Exists(path string) bool
	// ReadDir returns the file names in dir; a missing dir returns fs.ErrNotExist.
	ReadDir(dir string) ([]string, error)
	ReadFile(path string) (string, error)
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
	records, readWarnings := readRecords(root, effects)
	warnings = append(warnings, readWarnings...)
	for _, doc := range projectdocdomain.MatchRecords(normalizedTask, records, maxRoutedRecords) {
		entries = append(entries, projectdocscontract.ProjectDocRouteEntry{RelPath: doc.Rel, Path: effects.Path(root, doc.Rel), Reason: doc.Reason, Exists: true})
	}
	return projectdocscontract.ProjectDocsRouteResult{
		OK: true, Kind: "project_docs_route", RepoRoot: root, Task: normalizedTask,
		GeneratedAt: effects.Now().Format(time.RFC3339), Docs: entries, Warnings: warnings,
	}
}

// readRecords loads the dated records of every record family. The family
// indexes do not list records one by one, so routing reads them directly.
func readRecords(root string, effects RouteEffects) ([]projectdocdomain.Record, []string) {
	var records []projectdocdomain.Record
	var warnings []string
	for _, moduleDir := range projectdocdomain.RecordModuleDirs() {
		dirRel := path.Join(projectdocdomain.ProjectDocsDir, moduleDir)
		names, err := effects.ReadDir(effects.Path(root, dirRel))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			warnings = append(warnings, "could not list "+dirRel+" records: "+err.Error())
			continue
		}
		for _, name := range names {
			if !projectdocdomain.IsRecordFile(name) {
				continue
			}
			rel := path.Join(dirRel, name)
			content, err := effects.ReadFile(effects.Path(root, rel))
			if err != nil {
				warnings = append(warnings, "could not read record "+rel+": "+err.Error())
				continue
			}
			records = append(records, projectdocdomain.ParseRecord(rel, content))
		}
	}
	return records, warnings
}
