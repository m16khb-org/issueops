package projectdocs

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	projectdoc "issueops/internal/domain/projectdoc"
)

func AnalyzeProjectSignals(root string) projectdoc.ProjectSignals {
	files := listInterestingFiles(root)
	// Lists start empty so the bootstrap plan JSON never carries null lists.
	s := projectdoc.ProjectSignals{
		Files: append([]string{}, files...), Languages: []string{}, PackageManagers: []string{},
		TestCommands: []projectdoc.EvidenceCommand{}, BuildCommands: []projectdoc.EvidenceCommand{}, LintCommands: []projectdoc.EvidenceCommand{},
		ExistingAgentDocs: []string{}, GitHubWorkflows: []string{}, DetectedConventions: []string{},
	}
	for _, rel := range files {
		applyRootSignal(&s, rel)
		applyNestedManifestSignal(&s, rel)
		if strings.HasPrefix(rel, ".github/workflows/") {
			s.GitHubWorkflows = projectdoc.AppendUnique(s.GitHubWorkflows, rel)
		}
		if isPythonTestFile(rel) {
			// QA: a manifest-less Python repo (calc/, tests/test_*.py)
			// bootstrapped with "Could not auto-confirm languages".
			s.Languages = projectdoc.AppendUnique(s.Languages, "Python")
		}
	}
	applyDocumentedTestCommands(&s, root)
	sort.Strings(s.Languages)
	sort.Strings(s.PackageManagers)
	sort.Strings(s.ExistingAgentDocs)
	sort.Strings(s.GitHubWorkflows)
	sort.Strings(s.DetectedConventions)
	s.Profile = inferProjectProfile(root, s)
	return s
}

// applyRootSignal은 저장소 루트의 신호 파일 하나에서 얻은 언어·패키지 관리자·명령 근거와 컨벤션(Makefile/Taskfile), 기존 에이전트 문서(AGENTS.md/CLAUDE.md)를 기록한다.
func applyRootSignal(s *projectdoc.ProjectSignals, rel string) {
	addLang := func(v string) { s.Languages = projectdoc.AppendUnique(s.Languages, v) }
	addPM := func(v string) { s.PackageManagers = projectdoc.AppendUnique(s.PackageManagers, v) }
	addConvention := func(v string) { s.DetectedConventions = projectdoc.AppendUnique(s.DetectedConventions, v) }
	switch rel {
	case "go.mod":
		addLang("Go")
		s.TestCommands = append(s.TestCommands, projectdoc.EvidenceCommand{Command: "go test ./...", Evidence: []string{"go.mod"}, Confidence: "high"})
		// -o /dev/null: with one main package, plain `go build ./...` writes a
		// binary into the checkout (or fails when a directory has its name).
		s.BuildCommands = append(s.BuildCommands, projectdoc.EvidenceCommand{Command: "go build -o /dev/null ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
		s.LintCommands = append(s.LintCommands, projectdoc.EvidenceCommand{Command: "go vet ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
	case "package.json":
		addLang("JavaScript/TypeScript")
		addPM("npm-compatible")
		s.TestCommands = append(s.TestCommands, projectdoc.EvidenceCommand{Command: "npm test", Evidence: []string{"package.json"}, Confidence: "medium"})
	case "pnpm-lock.yaml":
		addPM("pnpm")
	case "yarn.lock":
		addPM("yarn")
	case "pyproject.toml":
		addLang("Python")
		addPM("pyproject")
		s.TestCommands = append(s.TestCommands, projectdoc.EvidenceCommand{Command: "pytest", Evidence: []string{"pyproject.toml"}, Confidence: "medium"})
	case "Cargo.toml":
		addLang("Rust")
		addPM("cargo")
		s.TestCommands = append(s.TestCommands, projectdoc.EvidenceCommand{Command: "cargo test", Evidence: []string{"Cargo.toml"}, Confidence: "high"})
		s.BuildCommands = append(s.BuildCommands, projectdoc.EvidenceCommand{Command: "cargo build", Evidence: []string{"Cargo.toml"}, Confidence: "high"})
	case "Makefile":
		addConvention("Makefile exists; inspect targets before inventing commands")
	case "Taskfile.yml", "Taskfile.yaml":
		addConvention("Taskfile exists; prefer documented task targets when present")
	case "AGENTS.md", "CLAUDE.md":
		s.ExistingAgentDocs = projectdoc.AppendUnique(s.ExistingAgentDocs, rel)
	}
}

// applyNestedManifestSignal은 하위 디렉터리 매니페스트에서 얻은 언어·패키지 관리자만 기록한다(명령 근거는 루트 신호만 만든다).
func applyNestedManifestSignal(s *projectdoc.ProjectSignals, rel string) {
	addLang := func(v string) { s.Languages = projectdoc.AppendUnique(s.Languages, v) }
	addPM := func(v string) { s.PackageManagers = projectdoc.AppendUnique(s.PackageManagers, v) }
	switch {
	case rel != "go.mod" && strings.HasSuffix(rel, "/go.mod"):
		addLang("Go")
	case rel != "package.json" && strings.HasSuffix(rel, "/package.json"):
		addLang("JavaScript/TypeScript")
		addPM("npm-compatible")
	case rel != "pyproject.toml" && strings.HasSuffix(rel, "/pyproject.toml"):
		addLang("Python")
		addPM("pyproject")
	case rel != "Cargo.toml" && strings.HasSuffix(rel, "/Cargo.toml"):
		addLang("Rust")
		addPM("cargo")
	}
}

func isPythonTestFile(rel string) bool {
	base := rel[strings.LastIndex(rel, "/")+1:]
	return strings.HasSuffix(base, ".py") && (strings.HasPrefix(base, "test_") || strings.HasSuffix(base, "_test.py"))
}

// documentedTestRunners are command prefixes that run a project's tests.
var documentedTestRunners = []string{
	"pytest", "python -m pytest", "python3 -m pytest", "python -m unittest", "python3 -m unittest",
	"python -B -m unittest", "python3 -B -m unittest", "go test", "npm test", "npm run test", "pnpm test",
	"yarn test", "cargo test", "make test", "prove", "bundle exec rspec", "rspec", "rake test", "deno test",
	"bun test", "mvn test", "gradle test", "./gradlew test", "dotnet test", "swift test", "mix test",
}

// applyDocumentedTestCommands adds test commands the root README.md or
// CONTRIBUTING.md names in inline code, so a repository without a manifest
// still gets the command its own docs give.
func applyDocumentedTestCommands(s *projectdoc.ProjectSignals, root string) {
	for _, name := range []string{"README.md", "CONTRIBUTING.md"} {
		for _, command := range documentedCommands(readSmallFile(filepath.Join(root, name), 64*1024)) {
			if !hasEvidenceCommand(s.TestCommands, command) {
				s.TestCommands = append(s.TestCommands, projectdoc.EvidenceCommand{Command: command, Evidence: []string{name}, Confidence: "medium"})
			}
		}
	}
}

func documentedCommands(text string) []string {
	var out []string
	pieces := strings.Split(text, "`")
	for i := 1; i < len(pieces); i += 2 {
		span := strings.TrimSpace(pieces[i])
		if span == "" || strings.Contains(span, "\n") {
			continue
		}
		for _, runner := range documentedTestRunners {
			if span == runner || strings.HasPrefix(span, runner+" ") {
				out = append(out, span)
				break
			}
		}
	}
	return out
}

func hasEvidenceCommand(commands []projectdoc.EvidenceCommand, command string) bool {
	for _, c := range commands {
		if c.Command == command {
			return true
		}
	}
	return false
}

func readSmallFile(path string, limit int64) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return ""
	}
	return string(data)
}
