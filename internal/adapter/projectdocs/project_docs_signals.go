package projectdocs

import (
	projectdoc "issueops/internal/domain/projectdoc"
	"sort"
	"strings"
)

func AnalyzeProjectSignals(root string) projectdoc.ProjectSignals {
	files := listInterestingFiles(root)
	s := projectdoc.ProjectSignals{Files: files}
	for _, rel := range files {
		applyRootSignal(&s, rel)
		applyNestedManifestSignal(&s, rel)
		if strings.HasPrefix(rel, ".github/workflows/") {
			s.GitHubWorkflows = projectdoc.AppendUnique(s.GitHubWorkflows, rel)
		}
	}
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
		s.BuildCommands = append(s.BuildCommands, projectdoc.EvidenceCommand{Command: "go build ./...", Evidence: []string{"go.mod"}, Confidence: "medium"})
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
