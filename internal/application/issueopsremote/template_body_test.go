package issueopsremote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateScoreSummaryReadsJSONOrAuthoredText(t *testing.T) {
	resolver := NewTemplateBodyResolver(os.ReadFile)
	for _, tc := range []struct{ name, body, want string }{
		{"scored JSON", `{"threshold":0.7,"selected_labels":[{"name":" bug ","score":0.95}]}`, "threshold 0.70\n선택 라벨: bug(0.95)"},
		{"authored summary", "  human summary\n", "human summary"},
		{"empty JSON", `{}`, "threshold 0.00"},
		{"null JSON", `null`, "threshold 0.00"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "scores")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := resolver.ScoreSummary(" " + path + " ")
			if err != nil || got != tc.want {
				t.Fatalf("summary = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if got, err := resolver.ScoreSummary("  "); err != nil || got != "" {
		t.Fatalf("no file = %q, %v", got, err)
	}
	if _, err := resolver.ScoreSummary(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatalf("missing file = %v", err)
	}
}
