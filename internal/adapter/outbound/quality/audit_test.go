package quality

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/quality"
)

const auditHeader = "| ID | Area | Title | Priority | Size |\n|----|------|-------|----------|------|\n"
const auditNone = "_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._"

func TestCollectAuditItemsValidDocuments(t *testing.T) {
	row := "| A1 | Core | Fix | P1 | Small |\n"
	for _, tc := range []struct {
		name, document string
		want           []contract.AuditItem
	}{
		{"priorities", auditHeader + "| A0 | Core | Urgent | P0 | Large |\n" + row + "| A2 | CLI | Check | P2 | Medium |\n| A3 | Docs | Later | P3 | Small |\n", []contract.AuditItem{{ID: "A0", Area: "Core", Title: "Urgent", Priority: "P0", Size: "Large"}, {ID: "A1", Area: "Core", Title: "Fix", Priority: "P1", Size: "Small"}, {ID: "A2", Area: "CLI", Title: "Check", Priority: "P2", Size: "Medium"}}},
		{"reordered extra columns", "| Note | priority | TITLE | Size | ID | area |\n|:---|---:|:---:|---|---|---|\n| extra | P1 | Fix | Small | A1 | Core |\n", []contract.AuditItem{{ID: "A1", Area: "Core", Title: "Fix", Priority: "P1", Size: "Small"}}},
		{"empty valid table", auditHeader, []contract.AuditItem{}},
		{"explicit zero", "## Summary Matrix\n### Open — counted\n" + auditNone + "\n### Resolved\n" + auditHeader + row, []contract.AuditItem{}},
		{"open scope", "# Audit\n## Open\n" + auditHeader + row + "### Details\nNotes\n## Accepted\n" + auditHeader + row + "## Resolved\n" + auditHeader + row + "## Out-of-scope\n" + auditHeader + row + "## Resolution Plan\n" + auditHeader + row, []contract.AuditItem{{ID: "A1", Area: "Core", Title: "Fix", Priority: "P1", Size: "Small"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", tc.document)
			items, warnings := CollectAuditItems(root)
			if len(warnings) != 0 || !reflect.DeepEqual(items, tc.want) {
				t.Fatalf("items=%+v warnings=%v want=%+v", items, warnings, tc.want)
			}
		})
	}
}

func TestCollectAuditItemsInvalidDocuments(t *testing.T) {
	row := "| A1 | Core | Fix | P1 | Small |\n"
	for _, tc := range []struct{ name, document string }{
		{"missing", ""},
		{"empty", " "},
		{"text only", "Audit completed"},
		{"headerless", row},
		{"missing header column", "| ID | Area | Title | Priority |\n|---|---|---|---|\n| A1 | Core | Fix | P1 |\n"},
		{"duplicate header", "| ID | Area | Title | Priority | Size | ID |\n|---|---|---|---|---|---|\n"},
		{"duplicate extra header", "| ID | Area | Title | Priority | Size | Note | note |\n|---|---|---|---|---|---|---|\n"},
		{"missing separator", "| ID | Area | Title | Priority | Size |\n" + row},
		{"invalid separator", "| ID | Area | Title | Priority | Size |\n|---|---|not separator|---|---|\n" + row},
		{"short separator", "| ID | Area | Title | Priority | Size |\n|---|---|---|---|\n" + row},
		{"short row", auditHeader + "| A1 | Core | Fix | P1 |\n"},
		{"long row", auditHeader + "| A1 | Core | Fix | P1 | Small | extra |\n"},
		{"invalid priority", auditHeader + "| A1 | Core | Fix | P9 | Small |\n"},
		{"none contradicts open", "## Open\n" + auditNone + "\n" + auditHeader + row},
		{"invalid open not rescued by other table", "# Audit\n## Open\nUnrecognized\n## Other\n" + auditHeader + row},
		{"history only", "# Audit\n## Resolved\n" + auditHeader + row},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.name != "missing" {
				writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", tc.document)
			}
			_, warnings := CollectAuditItems(root)
			if len(warnings) == 0 || !strings.Contains(warnings[0], filepath.Join(root, ".issueops", "PROJECT_AUDIT.md")) {
				t.Fatalf("missing path-qualified warning: %v", warnings)
			}
		})
	}
	for column := 0; column < 5; column++ {
		t.Run("empty required "+[]string{"ID", "Area", "Title", "Priority", "Size"}[column], func(t *testing.T) {
			root := t.TempDir()
			cells := []string{"A1", "Core", "Fix", "P1", "Small"}
			cells[column] = ""
			writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", auditHeader+"|"+strings.Join(cells, "|")+"|\n")
			_, warnings := CollectAuditItems(root)
			if len(warnings) == 0 {
				t.Fatal("empty required cell accepted")
			}
		})
	}
}

func TestCollectAuditItemsPreservesValidRowsWithWarnings(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", auditHeader+"| A1 | Core | Fix | P1 | Small |\n| A2 | Core | Broken | P9 | Small |\n")
	items, warnings := CollectAuditItems(root)
	if len(items) != 1 || items[0].ID != "A1" || len(warnings) == 0 {
		t.Fatalf("items=%+v warnings=%v", items, warnings)
	}
}

func TestCollectAuditItemsMissingLeadingPipe(t *testing.T) {
	root := t.TempDir()
	writeProductFixture(t, root, ".issueops/PROJECT_AUDIT.md", auditHeader+"A1 | Core | Fix | P0 | Small |\n")
	items, warnings := CollectAuditItems(root)
	if len(items) != 0 || len(warnings) == 0 || !strings.Contains(warnings[0], "PROJECT_AUDIT.md:3:") {
		t.Fatalf("malformed data row must warn: items=%+v warnings=%v", items, warnings)
	}
}

func TestCollectAuditItemsCurrentProjectDocument(t *testing.T) {
	items, warnings := CollectAuditItems(filepath.Join("..", "..", "..", ".."))
	if len(items) != 0 || len(warnings) != 0 {
		t.Fatalf("current audit document: items=%+v warnings=%v", items, warnings)
	}
}
