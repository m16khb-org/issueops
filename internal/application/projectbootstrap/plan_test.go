package projectbootstrap

import (
	"errors"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
	projectdoccontract "issueops/internal/contract/projectdoc"
	projectdoc "issueops/internal/domain/projectdoc"
)

var errDisk = errors.New("disk failed")

// memEffects is an in-memory repository: Exists/Action/Read reflect files and
// every Write is recorded and visible to later reads.
type memEffects struct {
	files      map[string]string
	rendered   map[string]string
	lifecycle  lifecyclecontract.ProjectLifecycleStatePlan
	initErr    error
	failWrite  string
	writes     []string
	initWrites []bool
	roots      []string
}

func (fake *memEffects) Analyze(root string) projectdoccontract.ProjectSignals {
	fake.roots = append(fake.roots, root)
	return projectdoccontract.ProjectSignals{}
}

func (fake *memEffects) InitLifecycle(_ string, write bool, _ projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	fake.initWrites = append(fake.initWrites, write)
	return fake.lifecycle, fake.initErr
}

func (fake *memEffects) Render(string, projectdoccontract.ProjectSignals) map[string]string {
	out := map[string]string{}
	maps.Copy(out, fake.rendered)
	return out
}
func (*memEffects) RenderAgents(_ string, existing string) string { return existing }
func (fake *memEffects) Exists(p string) bool {
	_, ok := fake.files[p]
	return ok
}

func (fake *memEffects) Action(p, content string) string {
	current, ok := fake.files[p]
	switch {
	case !ok:
		return "create"
	case current == content:
		return "unchanged"
	}
	return "update"
}

func (fake *memEffects) Write(p, content string) error {
	if p == fake.failWrite {
		return errDisk
	}
	if fake.files == nil {
		fake.files = map[string]string{}
	}
	fake.files[p] = content
	fake.writes = append(fake.writes, p)
	return nil
}

func (fake *memEffects) Read(p string) (string, error) {
	content, ok := fake.files[p]
	if !ok {
		return "", errDisk
	}
	return content, nil
}
func (*memEffects) Now() time.Time                   { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }
func (*memEffects) JoinPath(root, rel string) string { return path.Join(root, rel) }
func (*memEffects) ToSlash(rel string) string        { return rel }

func plannedByRel(result projectbootstrapcontract.ProjectDocsBootstrapResult, rel string) (projectdoccontract.ProjectDocsPlannedFile, bool) {
	for _, file := range result.Files {
		if file.RelPath == rel {
			return file, true
		}
	}
	return projectdoccontract.ProjectDocsPlannedFile{}, false
}

func TestServiceRunNormalizesRootBeforeBootstrapping(t *testing.T) {
	fake := &memEffects{}
	service := Service{NormalizeRoot: func(string) (string, error) { return "/canonical", nil }, Effects: fake}
	result, err := service.Run(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "relative"})
	if err != nil || result.RepoRoot != "/canonical" || !slices.Equal(fake.roots, []string{"/canonical"}) {
		t.Fatalf("result=%+v roots=%v err=%v", result, fake.roots, err)
	}

	rejecting := &memEffects{}
	_, err = Service{NormalizeRoot: func(string) (string, error) { return "", errDisk }, Effects: rejecting}.Run(projectbootstrapcontract.ProjectDocsBootstrapRequest{})
	if !errors.Is(err, errDisk) || len(rejecting.roots) != 0 {
		t.Fatalf("err=%v roots=%v: nothing may be analyzed when the root is rejected", err, rejecting.roots)
	}
}

func TestBootstrapPropagatesLifecycleFailureAndKeepsItsWarnings(t *testing.T) {
	fake := &memEffects{initErr: errDisk, rendered: map[string]string{"AGENTS.md": "agents"}}
	if _, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake); !errors.Is(err, errDisk) || len(fake.writes) != 0 {
		t.Fatalf("err=%v writes=%v", err, fake.writes)
	}

	fake = &memEffects{lifecycle: lifecyclecontract.ProjectLifecycleStatePlan{Warnings: []string{"state drift"}}}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo"}, fake)
	if err != nil || len(result.Warnings) < 2 || result.Warnings[0] != "state drift" ||
		!strings.HasPrefix(result.Warnings[len(result.Warnings)-1], "dry_run_only:") || !result.DryRun {
		t.Fatalf("warnings=%v dryRun=%v err=%v", result.Warnings, result.DryRun, err)
	}
	if !slices.Equal(fake.initWrites, []bool{false}) {
		t.Fatalf("lifecycle must see write=false on a dry run: %v", fake.initWrites)
	}
}

func TestBootstrapDryRunPlansEverythingButWritesNothing(t *testing.T) {
	fake := &memEffects{rendered: map[string]string{
		"AGENTS.md":                       "agents",
		".issueops/DESIGN.md":             "design",
		".issueops/ADR.md":                "adr root",
		".issueops/adr/overview.md":       "adr module",
		".issueops/EMPTY.md":              "",
		".issueops/operations/guides.md":  "ignored: not a planned path",
		".issueops/cautions/overview.md":  "cautions module",
		".issueops/testing/overview.md":   "testing module",
		".issueops/TESTING.md":            "testing root",
		".issueops/CONSTITUTION.md":       "",
		".issueops/architecture/notes.md": "ignored",
	}}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo"}, fake)
	if err != nil || len(fake.writes) != 0 {
		t.Fatalf("writes=%v err=%v", fake.writes, err)
	}
	wantReasons := map[string]string{
		"AGENTS.md":                             "agent entrypoint and routing block",
		".issueops/DESIGN.md":                   "client design system contract (client repositories only)",
		".issueops/ADR.md":                      "family root index linking its module directory",
		".issueops/adr/overview.md":             "family module starter document",
		".issueops/cautions/overview.md":        "family module starter document",
		".issueops/documentation/manifest.json": "modular documentation contract manifest",
	}
	for rel, reason := range wantReasons {
		file, ok := plannedByRel(result, rel)
		if !ok || file.Reason != reason || file.Action != "create" {
			t.Errorf("%s: planned=%+v ok=%v, want reason %q action create", rel, file, ok, reason)
		}
	}
	for _, rel := range []string{".issueops/EMPTY.md", ".issueops/CONSTITUTION.md", ".issueops/operations/guides.md", ".issueops/architecture/overview.md", ".issueops/architecture/notes.md"} {
		if _, ok := plannedByRel(result, rel); ok {
			t.Errorf("%s must not be planned (empty or unrendered content)", rel)
		}
	}
	if file, _ := plannedByRel(result, ".issueops/adr/overview.md"); file.Bytes != len("adr module") || file.SHA256 != projectdoc.SHA256Hex("adr module") {
		t.Errorf("planned file must describe the rendered content: %+v", file)
	}
}

func TestBootstrapWriteCreatesMissingFamilyModulesAndManifest(t *testing.T) {
	fake := &memEffects{rendered: map[string]string{
		".issueops/adr/overview.md": "adr module",
	}}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/repo/.issueops/adr/overview.md", "/repo/.issueops/documentation/manifest.json"} {
		if _, ok := fake.files[p]; !ok {
			t.Errorf("%s not created: writes=%v", p, fake.writes)
		}
	}
	if fake.files["/repo/.issueops/documentation/manifest.json"] != projectdoc.ManifestJSON() {
		t.Error("manifest content must be the domain manifest")
	}
	// Created docs are uncommitted; a cycle's worktree would not see them.
	if !slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.HasPrefix(w, "commit_before_cycles:") }) {
		t.Errorf("warnings = %v, want commit_before_cycles", result.Warnings)
	}
	again, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake)
	if err != nil || slices.ContainsFunc(again.Warnings, func(w string) bool { return strings.HasPrefix(w, "commit_before_cycles:") }) {
		t.Errorf("a run that creates nothing must not warn: %v %v", again.Warnings, err)
	}
}

func TestBootstrapPreservesExistingFamilyDocsEvenWithSync(t *testing.T) {
	fake := &memEffects{
		rendered: map[string]string{".issueops/adr/overview.md": "template", ".issueops/ADR.md": "template root"},
		files: map[string]string{
			"/repo/.issueops/adr/overview.md":             "curated module",
			"/repo/.issueops/ADR.md":                      "curated root",
			"/repo/.issueops/documentation/manifest.json": "{\"budgets\":1}",
		},
	}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true, Sync: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	if fake.files["/repo/.issueops/adr/overview.md"] != "curated module" || !strings.HasSuffix(fake.files["/repo/.issueops/ADR.md"], "curated root") || strings.Contains(fake.files["/repo/.issueops/ADR.md"], "template root") || fake.files["/repo/.issueops/documentation/manifest.json"] != "{\"budgets\":1}" {
		t.Fatalf("curated docs were overwritten: %v", fake.files)
	}
	for _, rel := range []string{".issueops/adr/overview.md", ".issueops/ADR.md"} {
		if file, ok := plannedByRel(result, rel); !ok || !file.Preserved || file.Action != "update" {
			t.Errorf("%s planned=%+v ok=%v, want preserved update", rel, file, ok)
		}
	}
	for _, prefix := range []string{"family_docs_preserved:", "manifest_preserved:"} {
		if !slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.HasPrefix(w, prefix) }) {
			t.Errorf("missing %s warning in %v", prefix, result.Warnings)
		}
	}
}

func TestBootstrapManifestPreservedWarningNeedsWriteAndSync(t *testing.T) {
	files := func() map[string]string {
		return map[string]string{"/repo/.issueops/documentation/manifest.json": "{}"}
	}
	for name, request := range map[string]projectbootstrapcontract.ProjectDocsBootstrapRequest{
		"write without sync": {RepoRoot: "/repo", Write: true},
		"sync without write": {RepoRoot: "/repo", Sync: true},
	} {
		result, err := Bootstrap(request, &memEffects{files: files()})
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.HasPrefix(w, "manifest_preserved:") }) {
			t.Errorf("%s: unexpected manifest_preserved warning %v", name, result.Warnings)
		}
	}
	result, _ := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true, Sync: true}, &memEffects{})
	if slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.HasPrefix(w, "manifest_preserved:") }) {
		t.Errorf("a manifest created by this run is not preserved: %v", result.Warnings)
	}
}

func countWarnings(warnings []string, prefix string) int {
	count := 0
	for _, warning := range warnings {
		if strings.HasPrefix(warning, prefix) {
			count++
		}
	}
	return count
}

func TestBootstrapWarnsAboutSyncOnceForSeveralExistingNonFamilyDocs(t *testing.T) {
	fake := &memEffects{
		rendered: map[string]string{".issueops/TECH_STACK.md": "new", ".issueops/VCS.md": "new"},
		files:    map[string]string{"/repo/.issueops/TECH_STACK.md": "old", "/repo/.issueops/VCS.md": "old"},
	}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	if n := countWarnings(result.Warnings, "sync_available:"); n != 1 {
		t.Fatalf("want exactly one deduplicated sync_available warning, got %d: %v", n, result.Warnings)
	}
	for _, rel := range []string{".issueops/TECH_STACK.md", ".issueops/VCS.md"} {
		if file, _ := plannedByRel(result, rel); !file.Preserved || file.Action != "update" {
			t.Fatalf("%s planned=%+v", rel, file)
		}
	}
}

func TestBootstrapDoesNotSuggestSyncForPreservedFamilyRootsOrCreates(t *testing.T) {
	fake := &memEffects{
		rendered: map[string]string{".issueops/ADR.md": "new root", ".issueops/VCS.md": "new"},
		files:    map[string]string{"/repo/.issueops/ADR.md": "curated"},
	}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	if n := countWarnings(result.Warnings, "sync_available:"); n != 0 {
		t.Fatalf("--sync never refreshes family roots and creates need no sync: %v", result.Warnings)
	}
	if countWarnings(result.Warnings, "family_docs_preserved:") != 1 {
		t.Fatalf("the preserved family root is reported instead: %v", result.Warnings)
	}
}

func TestBootstrapStopsAtFirstWriteFailure(t *testing.T) {
	cases := map[string]struct {
		fail     string
		rendered map[string]string
	}{
		"root doc":   {"/repo/AGENTS.md", map[string]string{"AGENTS.md": "agents"}},
		"family doc": {"/repo/.issueops/adr/overview.md", map[string]string{".issueops/adr/overview.md": "adr"}},
		"manifest":   {"/repo/.issueops/documentation/manifest.json", nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &memEffects{failWrite: tc.fail, rendered: tc.rendered}
			if _, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake); !errors.Is(err, errDisk) || len(fake.writes) != 0 {
				t.Fatalf("err=%v writes=%v: nothing may be written after the first failure", err, fake.writes)
			}
		})
	}
}

func TestBootstrapWriteBackfillsMissingMetaFrontmatterOnExistingDocs(t *testing.T) {
	fake := &memEffects{
		rendered: map[string]string{".issueops/TECH_STACK.md": "template"},
		files: map[string]string{
			"/repo/.issueops/TECH_STACK.md": "# Stack\n\nGo.\n",
			"/repo/.issueops/VCS.md":        projectdoc.EnsureMetaFrontmatter("VCS.md", "# VCS\n"),
		},
	}
	if _, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake); err != nil {
		t.Fatal(err)
	}
	want := projectdoc.EnsureMetaFrontmatter("TECH_STACK.md", "# Stack\n\nGo.\n")
	if want == "# Stack\n\nGo.\n" || fake.files["/repo/.issueops/TECH_STACK.md"] != want {
		t.Fatalf("curated doc must keep its body and gain frontmatter:\n got %q\nwant %q", fake.files["/repo/.issueops/TECH_STACK.md"], want)
	}
	if slices.Contains(fake.writes, "/repo/.issueops/VCS.md") {
		t.Fatalf("a doc whose frontmatter is already current must not be rewritten: %v", fake.writes)
	}

	// A dry run must not backfill.
	dry := &memEffects{files: map[string]string{"/repo/.issueops/TECH_STACK.md": "# Stack\n"}}
	if _, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo"}, dry); err != nil || len(dry.writes) != 0 {
		t.Fatalf("dry-run writes=%v err=%v", dry.writes, err)
	}
}

func TestBootstrapFrontmatterBackfillStopsOnWriteFailure(t *testing.T) {
	fake := &memEffects{
		failWrite: "/repo/.issueops/TECH_STACK.md",
		files:     map[string]string{"/repo/.issueops/TECH_STACK.md": "# Stack\n"},
	}
	if _, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: "/repo", Write: true}, fake); !errors.Is(err, errDisk) {
		t.Fatalf("err=%v", err)
	}
}
