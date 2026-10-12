package qagate

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

const badMermaidDoc = "```mermaid\nA[bad]\n```\n"

func relTestDeps(root, doc string, rel func(string, string) (string, error)) docsValidationDeps {
	return docsValidationDeps{
		listDocs: func(string) []string { return []string{doc} },
		readFile: func(string) ([]byte, error) { return []byte(badMermaidDoc), nil },
		exists:   func(string) bool { return true },
		glob:     func(string) ([]string, error) { return nil, nil },
		rel:      rel,
	}
}

func TestMermaidAndRedactionReportRootRelativePathWhenRelSucceeds(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, "docs", "a.md")
	deps := relTestDeps(root, doc, filepath.Rel)

	issues := validateMermaidDocsWithDeps(root, deps)
	if len(issues) != 1 || issues[0] != "docs/a.md:2 mermaid node text must start with a quote" {
		t.Fatalf("unexpected mermaid issues: %v", issues)
	}

	deps.readFile = func(string) ([]byte, error) { return []byte("password=hunter2hunter2\n"), nil }
	step := validateRedactionAuditWithDeps(root, deps)
	if step.OK || step.Error != "docs/a.md: line 1 contains secret_assignment" {
		t.Fatalf("unexpected redaction step: %+v", step)
	}
}

func TestMermaidAndRedactionFallBackToAbsolutePathWhenRelFails(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, "docs", "a.md")
	deps := relTestDeps(root, doc, func(string, string) (string, error) { return "", errors.New("bad rel") })

	issues := validateMermaidDocsWithDeps(root, deps)
	want := filepath.ToSlash(doc) + ":2 mermaid node text must start with a quote"
	if len(issues) != 1 || issues[0] != want {
		t.Fatalf("unexpected mermaid issues: %v, want %q", issues, want)
	}

	deps.readFile = func(string) ([]byte, error) { return []byte("password=hunter2hunter2\n"), nil }
	step := validateRedactionAuditWithDeps(root, deps)
	if step.OK || step.Error != filepath.ToSlash(doc)+": line 1 contains secret_assignment" {
		t.Fatalf("unexpected redaction step: %+v", step)
	}
}

func TestRedactionAuditDefaultRelReportsRootRelativePath(t *testing.T) {
	root := t.TempDir()
	doc := filepath.Join(root, "docs", "a.md")
	if err := os.MkdirAll(filepath.Dir(doc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doc, []byte("password=hunter2hunter2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := (Validator{ListDocs: func(string) []string { return []string{doc} }}).RedactionAudit(root)
	if step.OK || step.Error != "docs/a.md: line 1 contains secret_assignment" {
		t.Fatalf("unexpected step: %+v", step)
	}
}
