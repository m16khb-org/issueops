// Package projectbootstrap is the outbound adapter that gives the project docs
// bootstrap service its file-system effects: reading and writing documents,
// joining repository paths, and delegating repository analysis and rendering to
// injected functions.
package projectbootstrap

import (
	"os"
	"path/filepath"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

// Files implements the project docs bootstrap effects. The function fields are
// injected by the composition root; the remaining methods touch the file system
// or clock directly.
type Files struct {
	AnalyzeRepo         func(string) projectdoccontract.ProjectSignals
	InitializeLifecycle func(string, bool, ...projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error)
	RenderDocs          func(string, projectdoccontract.ProjectSignals) map[string]string
	RenderAgentsBlock   func(string, string) string
	FileAction          func(string, string) string
}

// Analyze returns the project signals detected for the repository at root.
func (files Files) Analyze(root string) projectdoccontract.ProjectSignals {
	return files.AnalyzeRepo(root)
}

// InitLifecycle plans (and, when write is set, writes) the project lifecycle
// state for root using profile.
func (files Files) InitLifecycle(root string, write bool, profile projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return files.InitializeLifecycle(root, write, profile)
}

// Render returns the generated project documents for root, keyed by relative
// path.
func (files Files) Render(root string, signals projectdoccontract.ProjectSignals) map[string]string {
	return files.RenderDocs(root, signals)
}

// RenderAgents returns the AGENTS.md content for root with its managed routing
// block merged into the existing content.
func (files Files) RenderAgents(root, existing string) string {
	return files.RenderAgentsBlock(root, existing)
}

// Exists reports whether path is an existing non-directory file.
func (files Files) Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Action returns the planned action name for writing content to path.
func (files Files) Action(path, content string) string { return files.FileAction(path, content) }
func (files Files) Write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
func (files Files) Read(path string) (string, error) {
	value, err := os.ReadFile(path)
	return string(value), err
}

// Now returns the current time.
func (files Files) Now() time.Time { return time.Now() }

// JoinPath joins the slash-separated relative path rel onto root using the
// operating system's separator.
func (files Files) JoinPath(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}

// ToSlash converts rel to slash separators.
func (files Files) ToSlash(rel string) string { return filepath.ToSlash(rel) }
