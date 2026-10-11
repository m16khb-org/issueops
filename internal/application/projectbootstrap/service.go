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
	run := &bootstrapRun{
		request:  request,
		effects:  effects,
		root:     root,
		files:    []projectdoccontract.ProjectDocsPlannedFile{},
		warnings: append([]string{}, lifecycleState.Warnings...),
	}
	run.contents = effects.Render(root, signals)
	run.contents["AGENTS.md"] = effects.RenderAgents(root, run.contents["AGENTS.md"])
	run.manifestPath = effects.JoinPath(root, projectdoc.ManifestRelPath())
	manifestExisted := effects.Exists(run.manifestPath)
	if err := run.planRootDocs(); err != nil {
		return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
	}
	if err := run.planFamilyOverviews(); err != nil {
		return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
	}
	if err := run.planManifest(); err != nil {
		return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
	}
	run.appendSummaryWarnings(manifestExisted)
	if request.Write {
		if err := run.ensureMetaFrontmatter(); err != nil {
			return projectbootstrapcontract.ProjectDocsBootstrapResult{}, err
		}
	}
	if !request.Write {
		run.warnings = append(run.warnings, "dry_run_only: rerun without --dry-run to create missing AGENTS.md/.issueops docs and repo metadata; add --sync to refresh existing docs")
	}
	return projectbootstrapcontract.ProjectDocsBootstrapResult{
		OK: true, Kind: "project_docs_bootstrap", RepoRoot: root, DocsDir: effects.JoinPath(root, projectdoc.ProjectDocsDir),
		Write: request.Write, Sync: request.Sync, DryRun: !request.Write, GeneratedAt: effects.Now().Format(time.RFC3339),
		Signals: signals, Files: run.files, LifecycleState: lifecycleState, Warnings: run.warnings,
	}, nil
}

// bootstrapRun accumulates the planned files and warnings of one Bootstrap call.
type bootstrapRun struct {
	request         projectbootstrapcontract.ProjectDocsBootstrapRequest
	effects         Effects
	root            string
	contents        map[string]string
	manifestPath    string
	files           []projectdoccontract.ProjectDocsPlannedFile
	warnings        []string
	familyPreserved bool
}

func (r *bootstrapRun) decide(kind projectdoc.BootstrapFileKind, rel, action string) projectdoc.BootstrapFileDecision {
	return projectdoc.DecideBootstrapFile(projectdoc.BootstrapFileInput{
		Kind: kind, Rel: rel, Action: action, Write: r.request.Write, Sync: r.request.Sync,
	})
}

func (r *bootstrapRun) planRootDocs() error {
	for _, rel := range append([]string{"AGENTS.md"}, projectdoc.PrefixedKnownProjectDocNames()...) {
		content := r.contents[rel]
		if content == "" {
			continue
		}
		path := r.effects.JoinPath(r.root, rel)
		action := r.effects.Action(path, content)
		decision := r.decide(projectdoc.BootstrapRootFile, rel, action)
		if decision.FamilyPreserved {
			r.familyPreserved = true
		}
		if decision.Write {
			if err := r.effects.Write(path, content); err != nil {
				return err
			}
		} else if decision.SyncWarning {
			r.warnings = projectdoc.AppendUnique(r.warnings, "sync_available: existing project docs were preserved; pass --sync to refresh them from current templates and repo evidence")
		}
		r.files = append(r.files, projectdoccontract.ProjectDocsPlannedFile{
			RelPath: r.effects.ToSlash(rel), Path: path, Action: action, Bytes: len([]byte(content)),
			SHA256: projectdoc.SHA256Hex(content), Reason: projectDocReason(rel), Preserved: decision.Preserved,
		})
	}
	return nil
}

func (r *bootstrapRun) planFamilyOverviews() error {
	for _, family := range projectdoc.DocFamilies() {
		rel := pathpkg.Join(projectdoc.ProjectDocsDir, family.OverviewRel())
		content := r.contents[rel]
		if content == "" {
			continue
		}
		path := r.effects.JoinPath(r.root, rel)
		action := r.effects.Action(path, content)
		decision := r.decide(projectdoc.BootstrapModuleFile, rel, action)
		if decision.Write {
			if err := r.effects.Write(path, content); err != nil {
				return err
			}
		} else if decision.FamilyPreserved {
			r.familyPreserved = true
		}
		if decision.Report {
			r.files = append(r.files, projectdoccontract.ProjectDocsPlannedFile{
				RelPath: rel, Path: path, Action: action, Bytes: len([]byte(content)),
				SHA256: projectdoc.SHA256Hex(content), Reason: projectDocReason(rel), Preserved: decision.Preserved,
			})
		}
	}
	return nil
}

func (r *bootstrapRun) planManifest() error {
	manifestContent := projectdoc.ManifestJSON()
	manifestAction := r.effects.Action(r.manifestPath, manifestContent)
	manifestDecision := r.decide(projectdoc.BootstrapManifestFile, projectdoc.ManifestRelPath(), manifestAction)
	if manifestDecision.Write {
		if err := r.effects.Write(r.manifestPath, manifestContent); err != nil {
			return err
		}
	}
	if manifestDecision.Report {
		r.files = append(r.files, projectdoccontract.ProjectDocsPlannedFile{
			RelPath: projectdoc.ManifestRelPath(), Path: r.manifestPath, Action: manifestAction,
			Bytes: len([]byte(manifestContent)), SHA256: projectdoc.SHA256Hex(manifestContent), Reason: "modular documentation contract manifest",
		})
	}
	return nil
}

func (r *bootstrapRun) appendSummaryWarnings(manifestExisted bool) {
	if r.familyPreserved {
		r.warnings = projectdoc.AppendUnique(r.warnings, "family_docs_preserved: modular family roots and module starters are never overwritten; revise them with project_docs_revise or reorganize with project-docs-optimize")
	}
	if manifestExisted && r.request.Write && r.request.Sync {
		r.warnings = projectdoc.AppendUnique(r.warnings, "manifest_preserved: documentation/manifest.json budgets are repo-owned; --sync does not reset them")
	}
}

// ensureMetaFrontmatter rewrites existing project docs whose meta frontmatter is missing or stale.
func (r *bootstrapRun) ensureMetaFrontmatter() error {
	for _, rel := range projectdoc.PrefixedKnownProjectDocNames() {
		path := r.effects.JoinPath(r.root, rel)
		existing, err := r.effects.Read(path)
		if err != nil {
			continue
		}
		ensured := projectdoc.EnsureMetaFrontmatter(pathpkg.Base(r.effects.ToSlash(rel)), existing)
		if ensured != existing {
			if err := r.effects.Write(path, ensured); err != nil {
				return err
			}
		}
	}
	return nil
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
