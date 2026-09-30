package issueopsreview

import (
	"strings"
	"testing"
)

func TestProjectDocumentPathPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, input, relative string
		changed               bool
		want                  string
	}{
		{"updated", "AGENTS.md", "AGENTS.md", true, ""},
		{"changed non-doc keeps existing contract", "src.go", "src.go", true, ""},
		{"outside precedes membership", "../outside", "", false, "must be inside"},
		{"unchanged", "AGENTS.md", "AGENTS.md", false, "not in the current change set"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateUpdatedDocumentPath(tc.input, tc.relative, tc.changed)
			if (err == nil) != (tc.want == "") || err != nil && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want=%q", err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, input, relative string
		root                  bool
		want                  string
	}{
		{"root doc", "AGENTS.md", "AGENTS.md", true, ""},
		{"nested doc", ".issueops/adr/decision.md", ".issueops/adr/decision.md", true, ""},
		{"outside precedes type and root", "../outside", "", false, "must be inside"},
		{"wrong type precedes missing root", "README.md", "README.md", false, "not a project doc"},
		{"directory prefix exact", ".issueops-other/doc.md", ".issueops-other/doc.md", true, "not a project doc"},
		{"nested AGENTS rejected", "src/AGENTS.md", "src/AGENTS.md", true, "not a project doc"},
		{"case sensitive", "agents.md", "agents.md", true, "not a project doc"},
		{"missing root", "AGENTS.md", "AGENTS.md", false, "without a worktree or repo root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReviewedDocumentPath(tc.input, tc.relative, tc.root)
			if (err == nil) != (tc.want == "") || err != nil && !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want=%q", err, tc.want)
			}
		})
	}
	if err := ValidateReviewedDocumentExists("AGENTS.md", false); err == nil || !strings.Contains(err.Error(), "does not exist as a file") {
		t.Fatalf("missing file: %v", err)
	}
	if err := ValidateReviewedDocumentExists("AGENTS.md", true); err != nil {
		t.Fatal(err)
	}
}
