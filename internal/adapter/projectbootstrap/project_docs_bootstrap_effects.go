package projectbootstrap

import (
	"os"
	"path/filepath"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

type bootstrapEffects struct{}

func (bootstrapEffects) Analyze(root string) projectdoccontract.ProjectSignals {
	return AnalyzeProjectSignals(root)
}
func (bootstrapEffects) InitLifecycle(root string, write bool, profile projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return initProjectLifecycleState(root, write, profile)
}
func (bootstrapEffects) Render(root string, signals projectdoccontract.ProjectSignals) map[string]string {
	return RenderProjectDocs(root, signals)
}
func (bootstrapEffects) RenderAgents(root, existing string) string {
	return RenderAgentsWithBlock(root, existing)
}
func (bootstrapEffects) Exists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
func (bootstrapEffects) Action(path, content string) string { return PlannedFileAction(path, content) }
func (bootstrapEffects) Write(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
func (bootstrapEffects) Read(path string) (string, error) {
	value, err := os.ReadFile(path)
	return string(value), err
}
func (bootstrapEffects) Now() time.Time { return time.Now() }

func (bootstrapEffects) JoinPath(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
func (bootstrapEffects) ToSlash(rel string) string { return filepath.ToSlash(rel) }
