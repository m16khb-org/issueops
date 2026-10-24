package remotecmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/cmd/issueops/jsonout"
	remoteapp "issueops/internal/application/issueopsremote"
	reportcontract "issueops/internal/contract/artifactreadability"
	issueopscontract "issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/domain/artifacttemplate"
	"issueops/internal/port"
	"issueops/internal/testsupport"
)

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	out, _ := testsupport.CaptureStdoutAndError(t, func() error { fn(); return nil })
	return out
}

type jsonSink struct {
	values []any
	errors []error
}

func (sink *jsonSink) deps() Deps {
	return Deps{
		PrintJSON:  func(value any) error { sink.values = append(sink.values, value); return nil },
		PrintError: func(err error) error { sink.errors = append(sink.errors, err); return nil },
	}
}

func stubCommand(ops RemoteDeps) Command {
	ops.IssueOpsStateRoot = func() string { return "/state-root" }
	return Command{Operations: ops}
}

func TestRemoteUnknownSubcommandIsRejected(t *testing.T) {
	err := testRemoteCommand().Run([]string{"frobnicate"}, Deps{})
	if err == nil || !strings.Contains(err.Error(), `unknown issueops remote subcommand "frobnicate"`) {
		t.Fatalf("expected unknown subcommand error, got %v", err)
	}
}

func TestRemoteSubcommandHelpAndFlagErrors(t *testing.T) {
	called := false
	command := stubCommand(RemoteDeps{
		MergeRemotePullRequest: func(context.Context, string, string, string, bool) (issueopscontract.RemoteMergeResult, error) {
			called = true
			return issueopscontract.RemoteMergeResult{}, nil
		},
	})
	var helpErr error
	captureStdout(t, func() { helpErr = command.Run([]string{"merge-pr", "-h"}, Deps{}) })
	if helpErr != nil {
		t.Fatalf("--help must not be an error, got %v", helpErr)
	}
	if err := command.Run([]string{"merge-pr", "--no-such-flag"}, Deps{}); err == nil {
		t.Fatal("unknown flag must be rejected")
	}
	if called {
		t.Fatal("operations must not run when flag parsing stopped the command")
	}
}

func TestRemoteMergePRTextOutputByOutcome(t *testing.T) {
	tests := []struct {
		name   string
		result issueopscontract.RemoteMergeResult
		err    error
		want   []string
		deny   []string
	}{
		{
			name:   "ready preview points at confirm",
			result: issueopscontract.RemoteMergeResult{OK: true, Provider: "github", URL: "https://x/pr/1", Method: "squash", State: "OPEN", Checks: "passing"},
			want:   []string{"github https://x/pr/1 state=OPEN", "ready to merge (squash); re-run with --confirm"},
			deny:   []string{"merged (", "blocked:"},
		},
		{
			name:   "merged prints commit",
			result: issueopscontract.RemoteMergeResult{OK: true, URL: "https://x/pr/1", Method: "merge", Merged: true, MergeCommitOID: " abc123\n"},
			want:   []string{"merged (merge) abc123"},
			deny:   []string{"ready to merge"},
		},
		{
			name:   "already merged wins over ready",
			result: issueopscontract.RemoteMergeResult{OK: true, URL: "https://x/pr/1", AlreadyMerged: true, Merged: true},
			want:   []string{"already merged"},
			deny:   []string{"ready to merge", "merged ("},
		},
		{
			name: "blockers and next command still print with the refusal error",
			result: issueopscontract.RemoteMergeResult{URL: "https://x/pr/1", Blockers: []issueopscontract.RemoteMergeBlocker{
				{Code: "checks_failing", Message: "ci red"},
			}, NextCommand: "issueops remote verify-artifact"},
			err:  errors.New("merge refused"),
			want: []string{"blocked: checks_failing: ci red", "next: issueops remote verify-artifact"},
			deny: []string{"ready to merge"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			command := stubCommand(RemoteDeps{
				MergeRemotePullRequest: func(context.Context, string, string, string, bool) (issueopscontract.RemoteMergeResult, error) {
					return test.result, test.err
				},
			})
			var runErr error
			out := captureStdout(t, func() { runErr = command.Run([]string{"merge-pr", "--id", "c1"}, Deps{}) })
			if !errors.Is(runErr, test.err) {
				t.Fatalf("error = %v, want %v", runErr, test.err)
			}
			for _, want := range test.want {
				if !strings.Contains(out, want) {
					t.Errorf("output missing %q:\n%s", want, out)
				}
			}
			for _, deny := range test.deny {
				if strings.Contains(out, deny) {
					t.Errorf("output must not contain %q:\n%s", deny, out)
				}
			}
		})
	}
}

func TestRemoteMergePRPassesFlagsAndRoutesErrors(t *testing.T) {
	var gotRoot, gotID, gotMethod string
	var gotConfirm bool
	var fail error
	var result issueopscontract.RemoteMergeResult
	command := stubCommand(RemoteDeps{
		MergeRemotePullRequest: func(_ context.Context, root, id, method string, confirm bool) (issueopscontract.RemoteMergeResult, error) {
			gotRoot, gotID, gotMethod, gotConfirm = root, id, method, confirm
			return result, fail
		},
	})
	sink := &jsonSink{}
	result = issueopscontract.RemoteMergeResult{OK: true, URL: "https://x/pr/9", Merged: true}
	if err := command.Run([]string{"merge-pr", "--id", "c9", "--method", "rebase", "--confirm", "--json"}, sink.deps()); err != nil {
		t.Fatal(err)
	}
	if gotRoot != "/state-root" || gotID != "c9" || gotMethod != "rebase" || !gotConfirm {
		t.Fatalf("flags not forwarded: root=%q id=%q method=%q confirm=%v", gotRoot, gotID, gotMethod, gotConfirm)
	}
	if len(sink.values) != 1 || sink.values[0].(issueopscontract.RemoteMergeResult).URL != "https://x/pr/9" {
		t.Fatalf("expected the merge result as JSON, got %#v", sink.values)
	}
	if err := command.Run([]string{"merge-pr", "--id", "c9"}, sink.deps()); err != nil {
		t.Fatal(err)
	}
	if gotMethod != issueopscontract.RemoteMergeMethodSquash || gotConfirm {
		t.Fatalf("defaults must be squash preview, got method=%q confirm=%v", gotMethod, gotConfirm)
	}

	// A refusal that carries a URL emits the observation with the error inside
	// it as the one JSON document and reports the error, so the root does not
	// print a second line under --json 2>&1.
	fail = errors.New("not mergeable")
	sink = &jsonSink{}
	if err := command.Run([]string{"merge-pr", "--id", "c9", "--json"}, sink.deps()); !errors.Is(err, fail) || !jsonout.IsReported(err) {
		t.Fatalf("expected reported refusal error, got %v", err)
	}
	if len(sink.values) != 1 || len(sink.errors) != 0 {
		t.Fatalf("observation must be printed as the result, not as an error payload: %#v %#v", sink.values, sink.errors)
	}
	if doc, _ := sink.values[0].(map[string]any); doc["url"] != "https://x/pr/9" || doc["error"] != "not mergeable" {
		t.Fatalf("refusal document = %#v", sink.values[0])
	}

	// Without a URL there is no observation, so the failure is the error payload.
	result = issueopscontract.RemoteMergeResult{}
	sink = &jsonSink{}
	if err := command.Run([]string{"merge-pr", "--id", "c9", "--json"}, sink.deps()); !errors.Is(err, fail) {
		t.Fatalf("expected error, got %v", err)
	}
	if len(sink.values) != 0 || len(sink.errors) != 1 {
		t.Fatalf("URL-less failure must print the error payload: %#v %#v", sink.values, sink.errors)
	}
}

func TestRemoteBodySyncMapsFlagsPerKindAndPrintsOutcome(t *testing.T) {
	var got remoteapp.BodySyncInput
	var result bodysynccontract.Result
	var fail error
	command := stubCommand(RemoteDeps{
		SyncRemoteBody: func(_ context.Context, root string, input remoteapp.BodySyncInput, _ remoteapp.AncestryObserver) (issueopscontract.IssueOpsRecord, bodysynccontract.Result, error) {
			got = input
			return issueopscontract.IssueOpsRecord{}, result, fail
		},
	})

	result = bodysynccontract.Result{Kind: "pr", URL: "https://x/pr/3", Drift: bodysynccontract.DriftInSync, RemoteBodySHA256: "r1", MergedBodySHA256: "m1"}
	out := captureStdout(t, func() {
		err := command.Run([]string{"sync-pr", "--id", "c3", "--expected-generation", "7", "--body", "new", "--host", "codex", "--session-id", "s", "--agent-id", "a", "--cwd", "/w", "--provider", "gitlab", "--accept-remote-edits", "--expected-body-sha256", "abc", "--confirm"}, Deps{})
		if err != nil {
			t.Fatal(err)
		}
	})
	if got.Command.Kind != bodysynccontract.KindPR || got.Command.ExpectedGeneration != 7 || got.Command.ID != "c3" || got.Command.ProposedBody != "new" ||
		!got.Command.Confirm || !got.Command.AcceptRemoteEdits || got.Command.ExpectedBodySHA256 != "abc" || got.Provider != "gitlab" ||
		got.Actor.Host != "codex" || got.Actor.SessionID != "s" || got.Actor.AgentID != "a" || got.Actor.CWD != "/w" {
		t.Fatalf("sync-pr flags not mapped: %#v", got)
	}
	if !strings.Contains(out, "kind=pr url=https://x/pr/3 drift="+string(bodysynccontract.DriftInSync)) || !strings.Contains(out, "already in sync; nothing to write") {
		t.Fatalf("in-sync output wrong:\n%s", out)
	}

	// sync-issue has no lease generation, so the flag must not exist for it.
	if err := command.Run([]string{"sync-issue", "--id", "c3", "--expected-generation", "7"}, Deps{}); err == nil {
		t.Fatal("sync-issue must reject --expected-generation")
	}

	result = bodysynccontract.Result{Kind: "issue", URL: "https://x/issues/3", Drift: bodysynccontract.DriftRemoteEdited, Updated: true, PreservedSections: []string{"Notes", "Links"}}
	out = captureStdout(t, func() {
		if err := command.Run([]string{"sync-issue", "--id", "c3", "--url", "https://x/issues/4", "--body-file", "b.md", "--template", "issue"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if got.Command.Kind != bodysynccontract.KindIssue || got.Command.URL != "https://x/issues/4" || got.BodyFile != "b.md" || got.Command.Template != "issue" || got.Command.ExpectedGeneration != 0 {
		t.Fatalf("sync-issue flags not mapped: %#v", got)
	}
	if !strings.Contains(out, "preserved: Notes, Links") || !strings.Contains(out, "body updated") {
		t.Fatalf("updated output wrong:\n%s", out)
	}

	result = bodysynccontract.Result{Kind: "issue", URL: "u", Drift: bodysynccontract.DriftRemoteEdited, ExpectedBodySHA256: "feed"}
	out = captureStdout(t, func() {
		if err := command.Run([]string{"sync-issue", "--id", "c3"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "preview only; re-run with --confirm --expected-body-sha256 feed") || strings.Contains(out, "preserved:") {
		t.Fatalf("preview output wrong:\n%s", out)
	}

	fail = errors.New("drift refused")
	sink := &jsonSink{}
	if err := command.Run([]string{"sync-issue", "--id", "c3", "--json"}, sink.deps()); !errors.Is(err, fail) {
		t.Fatalf("expected sync error, got %v", err)
	}
	if len(sink.errors) != 1 || len(sink.values) != 0 {
		t.Fatalf("failure must print an error payload only: %#v %#v", sink.values, sink.errors)
	}
	fail = nil
	result = bodysynccontract.Result{Kind: "issue", URL: "u"}
	if err := command.Run([]string{"sync-issue", "--id", "c3", "--json"}, sink.deps()); err != nil {
		t.Fatal(err)
	}
	if len(sink.values) != 1 {
		t.Fatalf("expected JSON result, got %#v", sink.values)
	}
}

func TestRemoteScorePromptAndJudgeFile(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "req.json")
	if err := os.WriteFile(input, []byte(`{"provider":"github","threshold":0.5,"issue":{"title":"Fix login bug","body":"login fails"},"issue_candidates":[{"id":"7","title":"Login bug","body":"same","score":0.9}],"label_candidates":[{"name":"bug","score":0.8}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	sink := &jsonSink{}
	var runErr error
	out := captureStdout(t, func() {
		runErr = testRemoteCommand().Run([]string{"score", "--input", input, "--judge", "prompt"}, sink.deps())
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(out, "Fix login bug") || len(sink.values) != 0 {
		t.Fatalf("prompt mode must print the rendered prompt as text:\n%s", out)
	}
	if err := testRemoteCommand().Run([]string{"score", "--input", input, "--judge", "prompt", "--json"}, sink.deps()); err != nil {
		t.Fatal(err)
	}
	if len(sink.values) != 1 {
		t.Fatalf("prompt --json must print one structured result, got %#v", sink.values)
	}

	// --judge file requires a path.
	err := testRemoteCommand().Run([]string{"score", "--input", input, "--judge", "file"}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "judge-file is required") {
		t.Fatalf("expected judge-file required error, got %v", err)
	}
	// Unreadable judge file.
	if err := testRemoteCommand().Run([]string{"score", "--input", input, "--judge", "file", "--judge-file", filepath.Join(dir, "absent.json")}, Deps{}); err == nil {
		t.Fatal("expected missing judge file error")
	}
	// A judge result that is not a valid host-agent scoring payload is rejected.
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"ok":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	err = testRemoteCommand().Run([]string{"score", "--input", input, "--judge", "file", "--judge-file", bad}, Deps{})
	if err == nil || !strings.Contains(err.Error(), "parse judge file "+bad) {
		t.Fatalf("expected wrapped judge parse error, got %v", err)
	}
	// A valid judge file is used verbatim for the summary.
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"ok":true,"provider":"gitlab","threshold":0.7,"execution_class":"background_join","read_only":true,"join_before":"remote_artifact_write","selected_related_issues":[{"id":"42","title":"Judged issue","score":0.91}],"selected_labels":[{"name":"judged-label","score":0.8}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out = captureStdout(t, func() {
		runErr = testRemoteCommand().Run([]string{"score", "--input", input, "--judge", "file", "--judge-file", good}, Deps{})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	for _, want := range []string{"provider=gitlab threshold=0.70 related_issues=1 labels=1", "- related issue: 42 (Judged issue) score=0.91", "- label: judged-label score=0.80"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
	// Malformed request JSON names the offending file.
	malformed := filepath.Join(dir, "malformed.json")
	if err := os.WriteFile(malformed, []byte(`{not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := testRemoteCommand().Run([]string{"score", "--input", malformed}, Deps{}); err == nil || !strings.Contains(err.Error(), "parse input file "+malformed) {
		t.Fatalf("expected parse input error, got %v", err)
	}
	if err := testRemoteCommand().Run([]string{"score"}, Deps{}); err == nil || !strings.Contains(err.Error(), "input is required") {
		t.Fatalf("expected input required error, got %v", err)
	}
}

func TestRemoteReflectCompletionBodyFileAndOutput(t *testing.T) {
	dir := t.TempDir()
	bodyPath := filepath.Join(dir, "report.md")
	if err := os.WriteFile(bodyPath, []byte("\n  progress report  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var gotBody, gotProvider string
	var gotConfirm bool
	var result port.IssueProviderUpdateIssueBodySectionResult
	var fail error
	command := stubCommand(RemoteDeps{
		ReflectRemoteCompletion: func(_ context.Context, _, _, provider, body string, confirm bool, _ remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error) {
			gotBody, gotProvider, gotConfirm = body, provider, confirm
			return issueopscontract.IssueOpsRecord{}, result, reportcontract.Report{}, fail
		},
	})

	result = port.IssueProviderUpdateIssueBodySectionResult{Updated: true, URL: "https://x/issues/1"}
	out := captureStdout(t, func() {
		if err := command.Run([]string{"reflect-completion", "--id", "c1", "--provider", "github", "--body-file", bodyPath, "--confirm"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if gotBody != "progress report" || gotProvider != "github" || !gotConfirm {
		t.Fatalf("body must be trimmed and flags forwarded: body=%q provider=%q confirm=%v", gotBody, gotProvider, gotConfirm)
	}
	if !strings.Contains(out, "reflected completion section: https://x/issues/1") {
		t.Fatalf("updated output wrong:\n%s", out)
	}

	result = port.IssueProviderUpdateIssueBodySectionResult{Preview: "would update section"}
	out = captureStdout(t, func() {
		if err := command.Run([]string{"reflect-completion", "--id", "c1"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if gotBody != "" || strings.TrimSpace(out) != "would update section" {
		t.Fatalf("preview without body file: body=%q out=%q", gotBody, out)
	}

	sink := &jsonSink{}
	if err := command.Run([]string{"reflect-completion", "--id", "c1", "--json"}, sink.deps()); err != nil {
		t.Fatal(err)
	}
	if len(sink.values) != 1 {
		t.Fatalf("expected JSON response, got %#v", sink.values)
	}
	if _, ok := sink.values[0].(reflectCompletionResponse); !ok {
		t.Fatalf("JSON must be the completion response with readability, got %T", sink.values[0])
	}

	// An unreadable body file fails before the operation runs.
	called := false
	command.Operations.ReflectRemoteCompletion = func(context.Context, string, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error) {
		called = true
		return issueopscontract.IssueOpsRecord{}, port.IssueProviderUpdateIssueBodySectionResult{}, reportcontract.Report{}, nil
	}
	sink = &jsonSink{}
	if err := command.Run([]string{"reflect-completion", "--id", "c1", "--body-file", filepath.Join(dir, "absent.md"), "--json"}, sink.deps()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected not-exist error, got %v", err)
	}
	if called || len(sink.errors) != 1 {
		t.Fatalf("operation must not run and error must be printed once: called=%v errors=%d", called, len(sink.errors))
	}

	fail = errors.New("merge unverified")
	command.Operations.ReflectRemoteCompletion = func(context.Context, string, string, string, string, bool, remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderUpdateIssueBodySectionResult, reportcontract.Report, error) {
		return issueopscontract.IssueOpsRecord{}, port.IssueProviderUpdateIssueBodySectionResult{}, reportcontract.Report{}, fail
	}
	sink = &jsonSink{}
	if err := command.Run([]string{"reflect-completion", "--id", "c1", "--json"}, sink.deps()); !errors.Is(err, fail) {
		t.Fatalf("expected operation error, got %v", err)
	}
	if len(sink.errors) != 1 || len(sink.values) != 0 {
		t.Fatalf("failure must print error payload only: %#v %#v", sink.values, sink.errors)
	}
}

func TestRemoteCloseIssueOutputAndErrors(t *testing.T) {
	var result port.IssueProviderCloseIssueResult
	var fail error
	var gotConfirm bool
	command := stubCommand(RemoteDeps{
		CloseRemoteIssue: func(_ context.Context, _, _, _ string, confirm bool, _ remoteapp.MergeVerifier) (issueopscontract.IssueOpsRecord, port.IssueProviderCloseIssueResult, error) {
			gotConfirm = confirm
			return issueopscontract.IssueOpsRecord{}, result, fail
		},
	})
	result = port.IssueProviderCloseIssueResult{Closed: true, IssueURL: "https://x/issues/5"}
	out := captureStdout(t, func() {
		if err := command.Run([]string{"close-issue", "--id", "c5", "--confirm"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if !gotConfirm || !strings.Contains(out, "closed issue: https://x/issues/5") {
		t.Fatalf("confirm=%v out=%q", gotConfirm, out)
	}
	result = port.IssueProviderCloseIssueResult{Preview: "would close issue"}
	out = captureStdout(t, func() {
		if err := command.Run([]string{"close-issue", "--id", "c5"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if gotConfirm || strings.TrimSpace(out) != "would close issue" {
		t.Fatalf("preview: confirm=%v out=%q", gotConfirm, out)
	}
	sink := &jsonSink{}
	if err := command.Run([]string{"close-issue", "--id", "c5", "--json"}, sink.deps()); err != nil || len(sink.values) != 1 {
		t.Fatalf("json result: err=%v values=%#v", err, sink.values)
	}
	fail = errors.New("merge not verified")
	sink = &jsonSink{}
	if err := command.Run([]string{"close-issue", "--id", "c5", "--json"}, sink.deps()); !errors.Is(err, fail) || len(sink.errors) != 1 || len(sink.values) != 0 {
		t.Fatalf("failure routing: err=%v values=%#v errors=%#v", err, sink.values, sink.errors)
	}
}

func TestRemoteSyncGraphTextOutput(t *testing.T) {
	var result map[string]any
	var fail error
	command := stubCommand(RemoteDeps{
		SyncIssueGraph: func(context.Context, string, string, bool) (map[string]any, error) { return result, fail },
	})
	run := func(args ...string) string {
		return captureStdout(t, func() {
			if err := command.Run(append([]string{"sync-graph", "--id", "c1"}, args...), Deps{}); err != nil {
				t.Fatal(err)
			}
		})
	}
	result = map[string]any{"link_count": 0}
	if out := run(); strings.TrimSpace(out) != "no issue graph links to sync" {
		t.Fatalf("empty dry-run output: %q", out)
	}
	result = map[string]any{"link_count": 2, "message": "would sync 2 links"}
	if out := run(); strings.TrimSpace(out) != "would sync 2 links" {
		t.Fatalf("dry-run output: %q", out)
	}
	if out := run("--confirm"); strings.TrimSpace(out) != "synced: 2 links" {
		t.Fatalf("confirm output: %q", out)
	}
	fail = errors.New("graph failed")
	sink := &jsonSink{}
	if err := command.Run([]string{"sync-graph", "--id", "c1", "--json"}, sink.deps()); !errors.Is(err, fail) || len(sink.errors) != 1 {
		t.Fatalf("failure routing: err=%v errors=%#v", err, sink.errors)
	}
}

func TestRemoteReconcileIssueOutputDistinguishesPreviewFromAdoption(t *testing.T) {
	var result issueopscontract.IssueOpsIssueCreateReconcileResult
	command := stubCommand(RemoteDeps{
		ReconcileIssueCreate: func(context.Context, string, string, bool, remoteapp.IssueLiveVerifier) (issueopscontract.IssueOpsIssueCreateReconcileResult, error) {
			return result, nil
		},
	})
	result = issueopscontract.IssueOpsIssueCreateReconcileResult{WouldAdopt: true, CandidateURL: "https://x/issues/8"}
	out := captureStdout(t, func() {
		if err := command.Run([]string{"reconcile-issue", "--id", "c8"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(out) != "would adopt: https://x/issues/8" {
		t.Fatalf("preview output: %q", out)
	}
	result = issueopscontract.IssueOpsIssueCreateReconcileResult{CandidateURL: "https://x/issues/8"}
	out = captureStdout(t, func() {
		if err := command.Run([]string{"reconcile-issue", "--id", "c8", "--confirm"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(out) != "adopted: https://x/issues/8" {
		t.Fatalf("adopt output: %q", out)
	}
}

func TestRemoteReconcileChildReportsErrorInJSONAndSuppressesTextOnFailure(t *testing.T) {
	var fail error
	command := stubCommand(RemoteDeps{
		ReconcileChild: func(context.Context, string, remoteapp.ChildReconcileCommand, remoteapp.AncestryObserver) (issueopscontract.ChildReconcileResult, error) {
			return issueopscontract.ChildReconcileResult{OperationID: "op1", ChildURL: "https://x/issues/2", WouldAdopt: true}, fail
		},
	})
	out := captureStdout(t, func() {
		if err := command.Run([]string{"reconcile-child", "--id", "c1", "--operation-id", "op1"}, Deps{}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.TrimSpace(out) != "child operation op1: https://x/issues/2 (preview=true)" {
		t.Fatalf("text output: %q", out)
	}
	fail = errors.New("ambiguous child")
	out = captureStdout(t, func() {
		if err := command.Run([]string{"reconcile-child", "--id", "c1", "--operation-id", "op1"}, Deps{}); !errors.Is(err, fail) {
			t.Fatalf("expected failure, got %v", err)
		}
	})
	if out != "" {
		t.Fatalf("text mode must not print success line on failure: %q", out)
	}
	sink := &jsonSink{}
	if err := command.Run([]string{"reconcile-child", "--id", "c1", "--operation-id", "op1", "--json"}, sink.deps()); !errors.Is(err, fail) {
		t.Fatalf("expected failure, got %v", err)
	}
	if len(sink.values) != 1 {
		t.Fatalf("JSON mode must still print the observation, got %#v", sink.values)
	}
}

func TestRemoteRenderTemplateRejectsMalformedFieldAndShowsMissingFields(t *testing.T) {
	sink := &jsonSink{}
	err := testRemoteCommand().Run([]string{"render-template", "--kind", "issue", "--template", "feature", "--title", "T", "--field", "no-equals-sign", "--json"}, sink.deps())
	if err == nil || len(sink.errors) != 1 {
		t.Fatalf("malformed field must fail and print error: err=%v errors=%d", err, len(sink.errors))
	}
	err = testRemoteCommand().Run([]string{"render-template", "--kind", "issue", "--template", "feature", "--title", "T", "--score-file", filepath.Join(t.TempDir(), "absent.json")}, Deps{})
	if err == nil {
		t.Fatal("unreadable score file must fail")
	}
	var runErr error
	out := captureStdout(t, func() {
		runErr = testRemoteCommand().Run([]string{"render-template", "--kind", "issue", "--template", "feature", "--title", "Only title", "--provider", "github"}, Deps{})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.HasPrefix(out, "# Only title\n") || !strings.Contains(out, "missing_required_fields: ") {
		t.Fatalf("rendering without fields must list missing required fields:\n%s", out)
	}
}

func TestObserveNativeProcessAncestryRejectsFailureAndEmptyReceipts(t *testing.T) {
	failure := errors.New("ps failed")
	_, err := Deps{ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, failure }}.observeNativeProcessAncestry()
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "observe native process ancestry") {
		t.Fatalf("expected wrapped observer failure, got %v", err)
	}
	_, err = Deps{ObserveProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, nil }}.observeNativeProcessAncestry()
	if err == nil || !strings.Contains(err.Error(), "no process receipts returned") {
		t.Fatalf("empty ancestry must be rejected, got %v", err)
	}
	receipts := []issueopscontract.NativeProcessReceipt{{PID: 1}}
	got, err := Deps{ObserveProcessAncestry: func(pid int) ([]issueopscontract.NativeProcessReceipt, error) {
		if pid != os.Getpid() {
			t.Fatalf("observer must receive own pid, got %d", pid)
		}
		return receipts, nil
	}}.observeNativeProcessAncestry()
	if err != nil || len(got) != 1 {
		t.Fatalf("expected receipts, got %v %v", got, err)
	}
}

func TestRemoteRenderTemplateOmitsMissingFieldsLineWhenAllRequiredFieldsAreSupplied(t *testing.T) {
	base := []string{"render-template", "--kind", "issue", "--template", "feature", "--title", "Complete", "--provider", "github"}
	sink := &jsonSink{}
	if err := testRemoteCommand().Run(append(append([]string{}, base...), "--json"), sink.deps()); err != nil {
		t.Fatal(err)
	}
	rendered, ok := sink.values[0].(artifacttemplate.IssueOpsTemplateResult)
	if !ok || len(rendered.MissingRequiredFields) == 0 {
		t.Fatalf("fixture needs required fields to supply, got %#v", sink.values[0])
	}
	args := append([]string{}, base...)
	for _, name := range rendered.MissingRequiredFields {
		args = append(args, "--field", name+"=value for "+name)
	}
	var runErr error
	out := captureStdout(t, func() { runErr = testRemoteCommand().Run(args, Deps{}) })
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.HasPrefix(out, "# Complete\n") || strings.Contains(out, "missing_required_fields") {
		t.Fatalf("rendering with every required field must not report missing fields:\n%s", out)
	}
}
