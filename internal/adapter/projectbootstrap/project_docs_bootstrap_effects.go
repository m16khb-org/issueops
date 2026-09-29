package projectbootstrap

import (
	"os"
	"path/filepath"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

type Files struct {
	AnalyzeRepo         func(string) projectdoccontract.ProjectSignals
	InitializeLifecycle func(string, bool, ...projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error)
	RenderDocs          func(string, projectdoccontract.ProjectSignals) map[string]string
	RenderAgentsBlock   func(string, string) string
	FileAction          func(string, string) string
}

func (files Files) Analyze(root string) projectdoccontract.ProjectSignals {
	return files.AnalyzeRepo(root)
}
func (files Files) InitLifecycle(root string, write bool, profile projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return files.InitializeLifecycle(root, write, profile)
}
func (files Files) Render(root string, signals projectdoccontract.ProjectSignals) map[string]string {
	return files.RenderDocs(root, signals)
}
func (files Files) RenderAgents(root, existing string) string {
	return files.RenderAgentsBlock(root, existing)
}
func (files Files) Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
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
func (files Files) Now() time.Time { return time.Now() }

func (files Files) JoinPath(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
func (files Files) ToSlash(rel string) string { return filepath.ToSlash(rel) }
