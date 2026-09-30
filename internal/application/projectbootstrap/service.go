package projectbootstrap

import (
	pathpkg "path"
	"strings"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
	projectdoccontract "issueops/internal/contract/projectdoc"
	projectdoc "issueops/internal/domain/projectdoc"
)

type Effects interface {
	Analyze(root string) projectdoccontract.ProjectSignals
	InitLifecycle(root string, write bool, profile projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error)
	Render(root string, signals projectdoccontract.ProjectSignals) map[string]string
	RenderAgents(root, existing string) string
	Exists(path string) bool
	Action(path, content string) string
	Write(path, content string) error
	Read(path string) (string, error)
	Now() time.Time
	JoinPath(root, rel string) string
	ToSlash(rel string) string
}

func Bootstrap(request projectbootstrapcontract.ProjectDocsBootstrapRequest, effects Effects) (projectbootstrapcontract.ProjectDocsBootstrapResult, error) {
	root := request.RepoRoot
	signals := effects.Analyze(root)
	lifecycleState, err := effects.InitLifecycle(root, request.Write, signals.Profile)
	if err != nil {
		return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
	}
	files := []projectdoc.ProjectDocsPlannedFile{}
	warnings := append([]string{}, lifecycleState.Warnings...)
	contents := effects.Render(root, signals)
	contents["AGENTS.md"] = effects.RenderAgents(root, contents["AGENTS.md"])
	manifestPath := effects.JoinPath(root, projectdoc.ManifestRelPath())
	manifestExisted := effects.Exists(manifestPath)
	legacyFlat := false
	if !manifestExisted {
		for _, family := range projectdoc.DocFamilies() {
			if effects.Exists(effects.JoinPath(root, pathpkg.Join(projectdoc.ProjectDocsDir, family.Root))) {
				legacyFlat = true
				break
			}
		}
	}
	if legacyFlat {
		warnings = projectdoc.AppendUnique(warnings, "legacy_flat_layout_preserved: existing flat family roots were kept without partial modular scaffolding; restructure with project-docs-optimize instead of bootstrap")
	}
	familyPreserved := false
	for _, rel := range append([]string{"AGENTS.md"}, projectdoc.PrefixedKnownProjectDocNames()...) {
		content := contents[rel]
		if content == "" {
			continue
		}
		path := effects.JoinPath(root, rel)
		action := effects.Action(path, content)
		decision := projectdoc.DecideBootstrapFile(projectdoc.BootstrapFileInput{
			Kind: projectdoc.BootstrapRootFile, Rel: rel, Action: action, Write: request.Write, Sync: request.Sync, LegacyFlat: legacyFlat,
		})
		if decision.FamilyPreserved {
			familyPreserved = true
		}
		if decision.Write {
			if err := effects.Write(path, content); err != nil {
				return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
			}
		} else if decision.SyncWarning {
			warnings = projectdoc.AppendUnique(warnings, "sync_available: existing project docs were preserved; pass --sync to refresh them from current templates and repo evidence")
		}
		files = append(files, projectdoc.ProjectDocsPlannedFile{
			RelPath: effects.ToSlash(rel), Path: path, Action: action, Bytes: len([]byte(content)),
			SHA256: projectdoc.SHA256Hex(content), Reason: projectDocReason(rel), Preserved: decision.Preserved,
		})
	}
	for _, family := range projectdoc.DocFamilies() {
		rel := pathpkg.Join(projectdoc.ProjectDocsDir, family.OverviewRel())
		content := contents[rel]
		if content == "" {
			continue
		}
		path := effects.JoinPath(root, rel)
		action := effects.Action(path, content)
		decision := projectdoc.DecideBootstrapFile(projectdoc.BootstrapFileInput{
			Kind: projectdoc.BootstrapModuleFile, Rel: rel, Action: action, Write: request.Write, Sync: request.Sync, LegacyFlat: legacyFlat,
		})
		if decision.Write {
			if err := effects.Write(path, content); err != nil {
				return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
			}
		} else if decision.FamilyPreserved {
			familyPreserved = true
		}
		if decision.Report {
			files = append(files, projectdoc.ProjectDocsPlannedFile{
				RelPath: rel, Path: path, Action: action, Bytes: len([]byte(content)),
				SHA256: projectdoc.SHA256Hex(content), Reason: projectDocReason(rel), Preserved: decision.Preserved,
			})
		}
	}
	manifestContent := projectdoc.ManifestJSON()
	manifestAction := effects.Action(manifestPath, manifestContent)
	manifestDecision := projectdoc.DecideBootstrapFile(projectdoc.BootstrapFileInput{
		Kind: projectdoc.BootstrapManifestFile, Rel: projectdoc.ManifestRelPath(), Action: manifestAction,
		Write: request.Write, Sync: request.Sync, LegacyFlat: legacyFlat,
	})
	if manifestDecision.Write {
		if err := effects.Write(manifestPath, manifestContent); err != nil {
			return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
		}
	}
	if manifestDecision.Report {
		files = append(files, projectdoc.ProjectDocsPlannedFile{
			RelPath: projectdoc.ManifestRelPath(), Path: manifestPath, Action: manifestAction,
			Bytes: len([]byte(manifestContent)), SHA256: projectdoc.SHA256Hex(manifestContent), Reason: "modular documentation contract manifest",
		})
	}
	if familyPreserved {
		warnings = projectdoc.AppendUnique(warnings, "family_docs_preserved: modular family roots and module starters are never overwritten; revise them with project_docs_revise or reorganize with project-docs-optimize")
	}
	if manifestExisted && request.Write && request.Sync {
		warnings = projectdoc.AppendUnique(warnings, "manifest_preserved: documentation/manifest.json budgets are repo-owned; --sync does not reset them")
	}
	if request.Write {
		for _, rel := range projectdoc.PrefixedKnownProjectDocNames() {
			path := effects.JoinPath(root, rel)
			existing, err := effects.Read(path)
			if err != nil {
				continue
			}
			ensured := projectdoc.EnsureMetaFrontmatter(pathpkg.Base(effects.ToSlash(rel)), existing)
			if ensured != existing {
				if err := effects.Write(path, ensured); err != nil {
					return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
				}
			}
		}
	}
	if !request.Write {
		warnings = append(warnings, "dry_run_only: rerun without --dry-run to create missing AGENTS.md/.issueops docs and repo metadata; add --sync to refresh existing docs")
	}
	return projectbootstrapcontract.ProjectDocsBootstrapResult{
		OK: true, Kind: "project_docs_bootstrap", RepoRoot: root, DocsDir: effects.JoinPath(root, projectdoc.ProjectDocsDir),
		Write: request.Write, Sync: request.Sync, DryRun: !request.Write, GeneratedAt: effects.Now().Format(time.RFC3339),
		Signals: signals, Files: files, LifecycleState: lifecycleState, Warnings: warnings,
	}, nil
}

func projectDocReason(rel string) string {
	if rel == "AGENTS.md" {
		return "agent entrypoint and routing block"
	}
	if rel == pathpkg.Join(projectdoc.ProjectDocsDir, "DESIGN.md") {
		return "client design system contract (client repositories only)"
	}
	if rel == projectdoc.ManifestRelPath() {
		return "modular documentation contract manifest"
	}
	stripped := strings.TrimPrefix(rel, projectdoc.ProjectDocsDir+"/")
	if _, ok := projectdoc.FamilyByRoot(stripped); ok {
		return "family root index linking its module directory"
	}
	if strings.HasPrefix(stripped, "adr/") || strings.HasPrefix(stripped, "architecture/") || strings.HasPrefix(stripped, "cautions/") || strings.HasPrefix(stripped, "conventions/") || strings.HasPrefix(stripped, "operations/") || strings.HasPrefix(stripped, "testing/") {
		return "family module starter document"
	}
	return "project-specific agent operating document"
}
