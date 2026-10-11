package projectcli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	commitsuggestcontract "issueops/internal/contract/commitsuggest"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
	projectdocscontract "issueops/internal/contract/projectdocs"
	"issueops/internal/testsupport"
)

func TestRun_routesEverySubcommandToItsHandler(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := newProjectAssistantGitRepo(t)
	deps := testProjectDependencies()
	cases := map[string]struct {
		args []string
		want string
	}{
		"docs":           {[]string{"docs", "--repo", repo}, ".issueops/TESTING.md — "},
		"route-docs":     {[]string{"route-docs", "--repo", repo, "--task", "test"}, "] — "},
		"append":         {[]string{"append", "--repo", repo, "--kind", "caution", "--title", "Dispatch", "--summary", "s"}, "appended caution in .issueops/cautions/"},
		"bootstrap":      {[]string{"bootstrap", "--repo", repo, "--dry-run"}, "project docs would update"},
		"lint-diagnose":  {[]string{"lint-diagnose", "--repo", repo, "--", "/bin/sh", "-c", "true"}, "No failure detected"},
		"commit-suggest": {[]string{"commit-suggest", "--repo", repo, "--json"}, `"executed": false`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := captureStatusVerifyStdout(t, func() error {
				_, err := captureProjectCLIStderr(t, func() error { return Run(deps, tc.args) })
				return err
			})
			if !strings.Contains(out, tc.want) {
				t.Fatalf("output lacks %q:\n%s", tc.want, out)
			}
		})
	}
}

func TestRunProjectSubcommands_rejectUnknownFlags(t *testing.T) {
	deps := testProjectDependencies()
	for _, sub := range []string{"bootstrap", "docs", "route-docs", "append", "commit-suggest", "lint-diagnose"} {
		t.Run(sub, func(t *testing.T) {
			_, err := captureProjectCLIStderr(t, func() error { return Run(deps, []string{sub, "--no-such-flag"}) })
			if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -no-such-flag") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestRunProjectBootstrap_writesDocsAndReportsThemInJSON(t *testing.T) {
	repo := t.TempDir()
	out := captureStatusVerifyStdout(t, func() error {
		return Run(testProjectDependencies(), []string{"bootstrap", "--repo", repo, "--json"})
	})
	var result projectbootstrapcontract.ProjectDocsBootstrapResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	if !result.Write || result.DryRun || result.Sync || len(result.Files) == 0 {
		t.Fatalf("result=%+v", result)
	}
	for _, rel := range []string{"AGENTS.md", ".issueops/documentation/manifest.json"} {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(rel))); err != nil {
			t.Errorf("--json bootstrap without --dry-run must write %s: %v", rel, err)
		}
	}
}

func TestRunProjectBootstrap_textReportsUpdatedFilesAndSyncFlag(t *testing.T) {
	repo := t.TempDir()
	out := captureStatusVerifyStdout(t, func() error {
		return runProjectBootstrap(testBootstrapService(), []string{"--sync", repo})
	})
	if !strings.Contains(out, "project docs updated ") || !strings.Contains(out, "- create AGENTS.md\n") {
		t.Fatalf("text output:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	// Second run: existing curated docs, so the planned actions are no longer creates.
	again := captureStatusVerifyStdout(t, func() error {
		return runProjectBootstrap(testBootstrapService(), []string{"--repo", repo, "--json", "--sync"})
	})
	var result projectbootstrapcontract.ProjectDocsBootstrapResult
	if err := json.Unmarshal([]byte(again), &result); err != nil || !result.Sync {
		t.Fatalf("sync flag must reach the request: %+v err=%v", result, err)
	}
	for _, file := range result.Files {
		if file.Action == "create" {
			t.Errorf("%s: a re-run over existing docs must not plan creates", file.RelPath)
		}
	}
}

func TestRunProjectBootstrap_failsForMissingRepo(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if err := runProjectBootstrap(testBootstrapService(), []string{"--repo", missing, "--dry-run"}); err == nil {
		t.Fatal("bootstrap of a missing repository must fail")
	}
	if _, err := os.Stat(missing); err == nil {
		t.Fatal("failed bootstrap must not create the repository")
	}
}

func TestRunProjectDocs_textListsRoutedDocsAndFailsForMissingRepo(t *testing.T) {
	repo := t.TempDir()
	out := captureStatusVerifyStdout(t, func() error {
		return runProjectDocs(testProjectDocsService(), []string{repo})
	})
	if !strings.Contains(out, ".issueops/TESTING.md — default test design and verification guidance\n") {
		t.Fatalf("text route must print `rel — reason` lines:\n%s", out)
	}
	if err := runProjectDocs(testProjectDocsService(), []string{filepath.Join(repo, "absent")}); err == nil {
		t.Fatal("route of a missing repository must fail")
	}
}

func TestRunProjectRouteDocs_textMarksExistingAndMissingDocs(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".issueops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".issueops", "TESTING.md"), []byte("# t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := captureStatusVerifyStdout(t, func() error {
		return runProjectRouteDocs(testProjectDocsService(), []string{"--repo", repo, "--task", "test"})
	})
	if !strings.Contains(out, ".issueops/TESTING.md [exists] — ") {
		t.Fatalf("existing doc must be marked exists:\n%s", out)
	}
	if !strings.Contains(out, ".issueops/TECH_STACK.md [missing] — toolchain evidence\n") {
		t.Fatalf("a doc absent on disk must be marked missing:\n%s", out)
	}
	if err := runProjectRouteDocs(testProjectDocsService(), []string{"--repo", filepath.Join(repo, "absent")}); err == nil {
		t.Fatal("route of a missing repository must fail")
	}
}

func TestRunProjectAppend_textPrintsSummaryAndWarnsOnStderrWhenOverviewIsMissing(t *testing.T) {
	repo := t.TempDir()
	var stderr string
	stdout := captureStatusVerifyStdout(t, func() error {
		var err error
		stderr, err = captureProjectCLIStderr(t, func() error {
			return runProjectAppend(testProjectDocsService(), []string{"--repo", repo, "--kind", "adr", "--title", "Plain text", "--summary", "s", "--source", "test"})
		})
		return err
	})
	if !strings.HasPrefix(stdout, "appended adr in .issueops/adr/") || !strings.Contains(stdout, "-plain-text.md (") || !strings.HasSuffix(stdout, " bytes)\n") {
		t.Fatalf("stdout=%q", stdout)
	}
	if !strings.Contains(stderr, "warning: .issueops/adr/overview.md is missing") || strings.Contains(stdout, "warning") {
		t.Fatalf("link warning must go to stderr only: stderr=%q stdout=%q", stderr, stdout)
	}
}

func TestRunProjectAppend_passesEveryFieldIntoTheRecord(t *testing.T) {
	repo := t.TempDir()
	out := captureStatusVerifyStdout(t, func() error {
		return runProjectAppend(testProjectDocsService(), []string{
			"--repo", repo, "--kind", "adr", "--title", "Fields", "--summary", "SUMMARY-X", "--context", "CONTEXT-X",
			"--decision", "DECISION-X", "--consequences", "CONSEQ-X", "--source", "SOURCE-X", "--json",
		})
	})
	var result projectdocscontract.ProjectDocsAppendResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode: %v\n%s", err, out)
	}
	body, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(result.RelPath)))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"SUMMARY-X", "CONTEXT-X", "DECISION-X", "CONSEQ-X", "SOURCE-X"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("record lacks %s:\n%s", want, body)
		}
	}

	caution := captureStatusVerifyStdout(t, func() error {
		return runProjectAppend(testProjectDocsService(), []string{"--repo", repo, "--kind", "caution", "--title", "Fix", "--summary", "s", "--resolution", "RESOLUTION-X", "--json"})
	})
	var cautionResult projectdocscontract.ProjectDocsAppendResult
	if err := json.Unmarshal([]byte(caution), &cautionResult); err != nil {
		t.Fatal(err)
	}
	body, err = os.ReadFile(filepath.Join(repo, filepath.FromSlash(cautionResult.RelPath)))
	if err != nil || !strings.Contains(string(body), "RESOLUTION-X") {
		t.Fatalf("caution lacks resolution: err=%v\n%s", err, body)
	}
}

func TestRunProjectCommitSuggest_printsPromptForStagedChanges(t *testing.T) {
	testsupport.IsolateGitConfig(t)
	repo := newProjectAssistantGitRepo(t)
	if err := os.WriteFile(filepath.Join(repo, "feature.txt"), []byte("brand new behavior\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runProjectAssistantGit(t, repo, "add", "feature.txt")

	out := captureStatusVerifyStdout(t, func() error {
		return runProjectCommitSuggest(testCommitService(), []string{"--repo", repo, "--staged"})
	})
	if !strings.Contains(out, "feature.txt") || !strings.Contains(out, "brand new behavior") {
		t.Fatalf("prompt must embed the staged diff:\n%s", out)
	}

	unstaged := captureStatusVerifyStdout(t, func() error {
		return runProjectCommitSuggest(testCommitService(), []string{"--repo", repo, "--json"})
	})
	var result commitsuggestcontract.CommitSuggestResult
	if err := json.Unmarshal([]byte(unstaged), &result); err != nil || result.Executed || result.Staged {
		t.Fatalf("without --staged the staged-only file is not part of the diff: %+v err=%v", result, err)
	}
}

func TestRunProjectCommitSuggest_failsForMissingRepo(t *testing.T) {
	err := runProjectCommitSuggest(testCommitService(), []string{"--repo", filepath.Join(t.TempDir(), "absent")})
	if err == nil {
		t.Fatal("expected an error for a missing repository")
	}
}

func TestRunProjectLintDiagnose_failsForMissingRepo(t *testing.T) {
	err := runProjectLintDiagnose(testLintService(), []string{"--repo", filepath.Join(t.TempDir(), "absent"), "--", "/bin/sh", "-c", "true"})
	if err == nil {
		t.Fatal("expected an error for a missing repository")
	}
}
