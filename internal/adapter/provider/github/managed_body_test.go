package github

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestGitHubCompletionSectionUsesDomainBudgetAndMerge(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX provider fixture")
	}
	for _, exhausted := range []bool{false, true} {
		name := "preserve authored bytes and truncate plan"
		if exhausted {
			name = "reject exhausted budget before edit"
		}
		t.Run(name, func(t *testing.T) {
			repo, bin := t.TempDir(), t.TempDir()
			prefix := "  authored\r\n\t"
			if exhausted {
				prefix = strings.Repeat("x", 60000-1000)
			}
			const suffix = "\r\n tail \n\n"
			const old = "<!-- issueops:completion:start -->old<!-- issueops:completion:end -->"
			payload, err := json.Marshal(map[string]string{"body": prefix + old + suffix})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(repo, "current.json"), payload, 0600); err != nil {
				t.Fatal(err)
			}
			writeFakeGh(t, bin, `#!/bin/sh
if [ "$1 $2" = "issue view" ]; then /bin/cat current.json; exit 0; fi
if [ "$1 $2" = "issue edit" ]; then printf '%s' "$5" > edited.body; exit 0; fi
exit 2
`)
			t.Setenv("PATH", bin)
			completion := model.RemoteCompletionSection{FinalHead: "abc123", PlanBody: strings.Repeat("p", 60000+1000), SpecBody: "preserved-spec", VerificationSummary: []string{strings.Repeat("v", 2000)}}
			result, err := NewProvider().UpdateIssueBodySection(port.IssueProviderUpdateIssueBodySectionRequest{Repo: repo, IssueURL: "https://github.com/acme/repo/issues/12", Section: model.IssueBodySectionCompletion, Completion: &completion, Confirm: true})
			path := filepath.Join(repo, "edited.body")
			if exhausted {
				if err == nil || !strings.Contains(err.Error(), "even after truncation") || result.Updated {
					t.Fatalf("budget refusal = %+v, %v", result, err)
				}
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("rejected body reached provider edit: %v", err)
				}
				return
			}
			if err != nil || !result.Updated {
				t.Fatalf("update = %+v, %v", result, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			body := string(data)
			if !strings.HasPrefix(body, prefix) || !strings.HasSuffix(body, suffix) {
				t.Fatal("authored bytes changed")
			}
			if strings.Contains(body, "old") || strings.Count(body, "<!-- issueops:completion:start -->") != 1 {
				t.Fatal("managed block was not replaced exactly once")
			}
			if !strings.Contains(body, "preserved-spec") || strings.Contains(body, strings.Repeat("p", 100)) || !strings.Contains(body, "일부 블록이 절단") || len(body) > 60000 {
				t.Fatal("completion truncation or budget not preserved")
			}
		})
	}
}
