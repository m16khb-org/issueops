package projectdocs

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

func RouteProjectDocs(repoRoot, task string) (projectdocscontract.ProjectDocsRouteResult, error) {
	root, err := NormalizeRepoRoot(repoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsRouteResult{}, err
	}
	normalizedTask := strings.ToLower(strings.TrimSpace(task))
	if normalizedTask == "" {
		normalizedTask = "general"
	}
	rels := projectdocdomain.RouteDocsForTask(normalizedTask)
	entries := make([]projectdocscontract.ProjectDocRouteEntry, 0, len(rels)*2)
	for _, rd := range rels {
		path := filepath.Join(root, filepath.FromSlash(rd.Rel))
		_, err := os.Stat(path)
		entries = append(entries, projectdocscontract.ProjectDocRouteEntry{RelPath: rd.Rel, Path: path, Reason: rd.Reason, Exists: err == nil})
		// Folder-first: when a routed doc is a family root and its overview
		// module exists, attach the module so agents read the actual detail,
		// not just the index.
		if family, ok := projectdocdomain.FamilyByRoot(filepath.Base(rd.Rel)); ok {
			ovRel := filepath.ToSlash(filepath.Join(ProjectDocsDir, family.OverviewRel()))
			ovPath := filepath.Join(root, filepath.FromSlash(ovRel))
			if _, err := os.Stat(ovPath); err == nil {
				entries = append(entries, projectdocscontract.ProjectDocRouteEntry{
					RelPath: ovRel,
					Path:    ovPath,
					Reason:  "family module detail for " + family.Root,
					Exists:  true,
				})
			}
		}
	}
	warnings := []string{}
	missingProjectDocs := true
	if _, err := os.Stat(filepath.Join(root, ProjectDocsDir)); err == nil {
		missingProjectDocs = false
	}
	if missingProjectDocs {
		warnings = append(warnings, "project docs are missing; run issueops project bootstrap to create AGENTS.md routing, .issueops docs, and repo metadata")
	}
	return projectdocscontract.ProjectDocsRouteResult{
		OK:          true,
		Kind:        "project_docs_route",
		RepoRoot:    root,
		Task:        normalizedTask,
		GeneratedAt: time.Now().Format(time.RFC3339),
		Docs:        entries,
		Warnings:    warnings,
	}, nil
}
