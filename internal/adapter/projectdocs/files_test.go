package projectdocs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	projectdoc "issueops/internal/domain/projectdoc"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAppendFilesAppendLineKeepsOneRecordPerLine(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct{ initial, want string }{
		"empty file":               {"", "- [a](a.md)\n"},
		"newline terminated file":  {"# Index\n", "# Index\n- [a](a.md)\n"},
		"missing trailing newline": {"# Index", "# Index\n- [a](a.md)\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, strings.ReplaceAll(name, " ", "_")+".md")
			writeFile(t, path, tc.initial)
			if err := (AppendFiles{}).AppendLine(path, "- [a](a.md)"); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, path); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAppendFilesAppendLineNeverCreatesTheFileAndReportsTheCause(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.md")
	err := (AppendFiles{}).AppendLine(path, "line")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("AppendLine must not create the overview: %v", statErr)
	}
}

func TestAppendFilesExistsTreatsOnlyNotExistAsAbsent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.md")
	writeFile(t, file, "x")
	if !(AppendFiles{}).Exists(file) || !(AppendFiles{}).Exists(dir) {
		t.Fatal("existing file and directory must exist")
	}
	if (AppendFiles{}).Exists(filepath.Join(dir, "absent.md")) {
		t.Fatal("absent path must not exist")
	}
	// A path through a regular file fails with ENOTDIR, not ErrNotExist: the
	// record name is then treated as taken so it is never overwritten.
	if !(AppendFiles{}).Exists(filepath.Join(file, "child.md")) {
		t.Fatal("a stat failure other than not-exist must count as existing")
	}
}

func TestAppendFilesEnsureDirWriteAndPath(t *testing.T) {
	root := t.TempDir()
	files := AppendFiles{}
	dir := files.Path(root, ".issueops/adr")
	if dir != filepath.Join(root, ".issueops", "adr") {
		t.Fatalf("Path=%q", dir)
	}
	if err := files.EnsureDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := files.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir must be idempotent: %v", err)
	}
	record := filepath.Join(dir, "r.md")
	if err := files.Write(record, "body"); err != nil || readFile(t, record) != "body" {
		t.Fatalf("write err=%v", err)
	}
	if err := files.Write(filepath.Join(dir, "missing", "r.md"), "x"); err == nil {
		t.Fatal("Write must not create parent directories")
	}
}

func TestRouteFilesListsOnlyRegularFilesAndSignalsMissingDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.md"), "a")
	writeFile(t, filepath.Join(dir, "sub", "nested.md"), "n")
	if err := os.Symlink(filepath.Join(dir, "a.md"), filepath.Join(dir, "link.md")); err != nil {
		t.Fatal(err)
	}
	names, err := (RouteFiles{}).ReadDir(dir)
	if err != nil || !slices.Equal(names, []string{"a.md"}) {
		t.Fatalf("names=%v err=%v: directories and symlinks are not records", names, err)
	}
	if _, err := (RouteFiles{}).ReadDir(filepath.Join(dir, "absent")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v: the application layer relies on fs.ErrNotExist for a missing family dir", err)
	}
}

func TestRouteFilesReadFileAndExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.md")
	writeFile(t, path, "# Record\n")
	if got, err := (RouteFiles{}).ReadFile(path); err != nil || got != "# Record\n" {
		t.Fatalf("got=%q err=%v", got, err)
	}
	if _, err := (RouteFiles{}).ReadFile(filepath.Join(dir, "absent.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err=%v", err)
	}
	if !(RouteFiles{}).Exists(path) || (RouteFiles{}).Exists(filepath.Join(dir, "absent.md")) {
		t.Fatal("Exists must reflect the filesystem")
	}
	if (RouteFiles{}).Path("/r", ".issueops/TESTING.md") != filepath.Join("/r", ".issueops", "TESTING.md") {
		t.Fatal("Path must join the slash-separated rel onto the root")
	}
}

func TestRevisionFilesReadDistinguishesMissingFromUnreadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.md")
	writeFile(t, path, "")
	if content, exists, err := (RevisionFiles{}).Read(path); err != nil || !exists || content != "" {
		t.Fatalf("empty file: content=%q exists=%v err=%v", content, exists, err)
	}
	if content, exists, err := (RevisionFiles{}).Read(filepath.Join(dir, "absent.md")); err != nil || exists || content != "" {
		t.Fatalf("missing file: content=%q exists=%v err=%v", content, exists, err)
	}
	if _, exists, err := (RevisionFiles{}).Read(dir); err == nil || exists {
		t.Fatalf("a directory is unreadable, not missing: exists=%v err=%v", exists, err)
	}
}

func TestRevisionFilesWriteCreatesParentsAndFailsUnderAFile(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, ".issueops", "adr", "r.md")
	if err := (RevisionFiles{}).Write(nested, "x"); err != nil || readFile(t, nested) != "x" {
		t.Fatalf("err=%v", err)
	}
	blocker := filepath.Join(dir, "blocker")
	writeFile(t, blocker, "file")
	if err := (RevisionFiles{}).Write(filepath.Join(blocker, "r.md"), "x"); err == nil {
		t.Fatal("writing beneath a regular file must fail")
	}
	if got := readFile(t, blocker); got != "file" {
		t.Fatalf("blocker clobbered: %q", got)
	}
}

func TestAnalyzeProjectSignalsRecordsEvidencePerManifest(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{
		"pnpm-lock.yaml", "yarn.lock", "pyproject.toml", "Cargo.toml", "Makefile", "Taskfile.yaml", "CLAUDE.md",
		".github/workflows/ci.yml",
		"services/api/go.mod", "web/app/package.json", "tools/py/pyproject.toml", "crates/x/Cargo.toml",
		"node_modules/dep/package.json", "vendor/mod/go.mod", "a/b/c/d/e/go.mod",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), "")
	}
	signals := AnalyzeProjectSignals(root)

	if want := []string{"Go", "JavaScript/TypeScript", "Python", "Rust"}; !slices.Equal(signals.Languages, want) {
		t.Fatalf("languages=%v, want %v (nested manifests count, skipped dirs do not)", signals.Languages, want)
	}
	if want := []string{"cargo", "npm-compatible", "pnpm", "pyproject", "yarn"}; !slices.Equal(signals.PackageManagers, want) {
		t.Fatalf("package managers=%v, want %v", signals.PackageManagers, want)
	}
	if slices.ContainsFunc(signals.Files, func(rel string) bool {
		return strings.HasPrefix(rel, "node_modules/") || strings.HasPrefix(rel, "vendor/") || strings.HasPrefix(rel, "a/b/c/d/")
	}) {
		t.Fatalf("skipped and too-deep files must not be signals: %v", signals.Files)
	}
	commands := func(list []projectdoc.EvidenceCommand) []string {
		var out []string
		for _, c := range list {
			out = append(out, c.Command)
		}
		return out
	}
	if got := commands(signals.TestCommands); !slices.Equal(got, []string{"cargo test", "pytest"}) {
		t.Fatalf("test commands=%v: only root manifests propose commands", got)
	}
	if got := commands(signals.BuildCommands); !slices.Equal(got, []string{"cargo build"}) {
		t.Fatalf("build commands=%v", got)
	}
	if len(signals.LintCommands) != 0 {
		t.Fatalf("lint commands come only from a root go.mod: %v", signals.LintCommands)
	}
	if want := []string{"CLAUDE.md"}; !slices.Equal(signals.ExistingAgentDocs, want) {
		t.Fatalf("agent docs=%v", signals.ExistingAgentDocs)
	}
	if want := []string{".github/workflows/ci.yml"}; !slices.Equal(signals.GitHubWorkflows, want) {
		t.Fatalf("workflows=%v", signals.GitHubWorkflows)
	}
	if len(signals.DetectedConventions) != 2 || !strings.HasPrefix(signals.DetectedConventions[0], "Makefile exists") || !strings.HasPrefix(signals.DetectedConventions[1], "Taskfile exists") {
		t.Fatalf("conventions=%v", signals.DetectedConventions)
	}
}

func TestAnalyzeProjectSignalsRootGoModProposesGoCommands(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module x\n")
	signals := AnalyzeProjectSignals(root)
	if len(signals.TestCommands) != 1 || signals.TestCommands[0].Command != "go test ./..." || signals.TestCommands[0].Confidence != "high" ||
		len(signals.BuildCommands) != 1 || len(signals.LintCommands) != 1 || signals.LintCommands[0].Command != "go vet ./..." {
		t.Fatalf("signals=%+v", signals)
	}
	// Root and nested go.mod must not double-count the language.
	writeFile(t, filepath.Join(root, "svc", "go.mod"), "module y\n")
	if got := AnalyzeProjectSignals(root).Languages; !slices.Equal(got, []string{"Go"}) {
		t.Fatalf("languages=%v", got)
	}
}

func TestEnsureBehavioralGuidelinesAtTopOnlyPrependsToHeadinglessText(t *testing.T) {
	template := strings.TrimRight(projectdoc.BehavioralGuidelines, "\n")
	if got := ensureBehavioralGuidelinesAtTop("\ufeff\n  " + template + "\n"); !strings.Contains(got, "\ufeff") || strings.Count(got, "Behavioral guidelines to reduce") != 1 {
		t.Fatalf("leading BOM/whitespace must not cause a second template: %q", got)
	}
	if got := ensureBehavioralGuidelinesAtTop("# Own rules\n"); got != "# Own rules\n" {
		t.Fatalf("a curated heading is authoritative: %q", got)
	}
	got := ensureBehavioralGuidelinesAtTop("\n\nplain notes\n")
	if want := template + "\n\n---\n\nplain notes\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderAgentsWithBlockAppendsBlockToExistingFileWithoutMarkers(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "AGENTS.md"), "# Own\n\nrules\n")
	got := RenderAgentsWithBlock(root, "")
	if !strings.HasPrefix(got, "# Own\n\nrules\n\n"+projectdoc.AgentsStartMarker) || !strings.HasSuffix(got, projectdoc.AgentsEndMarker+"\n") {
		t.Fatalf("block must be appended after the curated text:\n%s", got)
	}
	if strings.Count(got, projectdoc.AgentsStartMarker) != 1 {
		t.Fatalf("exactly one block expected:\n%s", got)
	}
	// Refreshing is idempotent once the block exists.
	writeFile(t, filepath.Join(root, "AGENTS.md"), got)
	if again := RenderAgentsWithBlock(root, ""); again != got {
		t.Fatalf("refresh changed an up-to-date file:\n%s", again)
	}
}

func TestFamilyHelpersRejectUnknownInputs(t *testing.T) {
	if got := familyOverviewBody("NOPE.md", projectdoc.ProjectSignals{}); got != "" {
		t.Fatalf("unknown family root must render nothing: %q", got)
	}
	if capitalize("") != "" || capitalize("abc") != "Abc" {
		t.Fatal("capitalize must upper-case only the first byte and accept empty input")
	}
}

func TestReadGitOriginURLReadsOnlyTheOriginRemote(t *testing.T) {
	root := t.TempDir()
	if got := ReadGitOriginURL(root); got != "" {
		t.Fatalf("no .git/config: %q", got)
	}
	writeFile(t, filepath.Join(root, ".git", "config"), "[core]\n\turl = not-a-remote\n[remote \"upstream\"]\n\turl = https://example.com/up.git\n[remote \"origin\"]\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n\turl = https://example.com/origin.git\n[branch \"main\"]\n\turl = ignored\n")
	if got := ReadGitOriginURL(root); got != "https://example.com/origin.git" {
		t.Fatalf("got %q", got)
	}
	writeFile(t, filepath.Join(root, ".git", "config"), "[remote \"origin\"]\n\turl\n")
	if got := ReadGitOriginURL(root); got != "" {
		t.Fatalf("a url key without a value yields nothing: %q", got)
	}
}

// QA: a manifest-less Python repository bootstrapped with no language and no
// test command although its README named the command. Test files mark the
// language; README/CONTRIBUTING code spans give the documented command.
func TestAnalyzeProjectSignalsReadsDocumentedTestCommandAndPythonTests(t *testing.T) {
	root := t.TempDir()
	for rel, body := range map[string]string{
		"README.md":          "# calc\n\nTests: `python3 -B -m unittest discover -s tests`\n\n```bash\npytest -x\n```\nUse `make` and `go testify`.\n",
		"CONTRIBUTING.md":    "Run `python3 -B -m unittest discover -s tests` and `cargo test --all` first.\n",
		"tests/test_calc.py": "import unittest\n",
		"calc/__init__.py":   "def add(a, b): return a + b\n",
	} {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	signals := AnalyzeProjectSignals(root)
	if !slices.Equal(signals.Languages, []string{"Python"}) {
		t.Fatalf("languages = %v", signals.Languages)
	}
	var got []string
	for _, c := range signals.TestCommands {
		got = append(got, c.Command+"@"+c.Evidence[0])
	}
	if want := []string{"python3 -B -m unittest discover -s tests@README.md", "cargo test --all@CONTRIBUTING.md"}; !slices.Equal(got, want) {
		t.Fatalf("test commands = %v, want %v", got, want)
	}
}
