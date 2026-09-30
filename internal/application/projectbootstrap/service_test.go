package projectbootstrap

import (
	"errors"
	"path"
	"strings"
	"testing"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

type bootstrapEffects struct {
	writes   []string
	existing map[string]bool
}

func (*bootstrapEffects) Analyze(string) projectdoccontract.ProjectSignals {
	return projectdoccontract.ProjectSignals{}
}
func (*bootstrapEffects) InitLifecycle(string, bool, projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	return lifecyclecontract.ProjectLifecycleStatePlan{}, nil
}
func (*bootstrapEffects) Render(string, projectdoccontract.ProjectSignals) map[string]string {
	return map[string]string{"AGENTS.md": "agents", ".issueops/TECH_STACK.md": "new template"}
}
func (*bootstrapEffects) RenderAgents(_ string, existing string) string { return existing }
func (fake *bootstrapEffects) Exists(path string) bool                  { return fake.existing[path] }
func (fake *bootstrapEffects) Action(path, _ string) string {
	if fake.existing[path] {
		return "update"
	}
	return "create"
}
func (fake *bootstrapEffects) Write(path, _ string) error {
	fake.writes = append(fake.writes, path)
	return nil
}
func (*bootstrapEffects) Read(string) (string, error) {
	return "", errors.New("no frontmatter fixture")
}
func (*bootstrapEffects) Now() time.Time { return time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) }

func (*bootstrapEffects) JoinPath(root, rel string) string { return path.Join(root, rel) }
func (*bootstrapEffects) ToSlash(rel string) string        { return rel }

func TestBootstrapPreservesCuratedDocsWithoutSync(t *testing.T) {
	root := "/repo"
	path := root + "/.issueops/TECH_STACK.md"
	fake := &bootstrapEffects{existing: map[string]bool{path: true}}
	result, err := Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: root, Write: true}, fake)
	if err != nil || !result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	for _, written := range fake.writes {
		if written == path {
			t.Fatalf("curated doc overwritten: %v", fake.writes)
		}
	}
	fake.writes = nil
	_, err = Bootstrap(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: root, Write: true, Sync: true}, fake)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, written := range fake.writes {
		if written == path {
			found = true
		}
	}
	if !found {
		t.Fatalf("sync did not update standard doc: %v", fake.writes)
	}
	if !strings.HasPrefix(result.GeneratedAt, "2000-01-01") {
		t.Fatalf("generated_at=%q", result.GeneratedAt)
	}
}
