package feedbackcleanup

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	cleanupapp "issueops/internal/application/issueopscleanup"
	issueopscontract "issueops/internal/contract/issueops"
	orphancontract "issueops/internal/contract/issueopsorphancleanup"
	"issueops/internal/port"
)

// captureCleanupStdout runs fn while os.Stdout is redirected and returns what
// the command rendered. Tests using it must not run in parallel.
func captureCleanupStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	runErr := func() error {
		defer func() { os.Stdout = original }()
		return fn()
	}()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out), runErr
}

// cleanupOutputs records what the command asked the CLI shell to print.
type cleanupOutputs struct {
	json    []any
	errors  []error
	jsonErr error
	errErr  error
}

func (o *cleanupOutputs) deps() Deps {
	return Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		PrintJSON: func(value any) error {
			o.json = append(o.json, value)
			return o.jsonErr
		},
		PrintError: func(err error) error {
			o.errors = append(o.errors, err)
			return o.errErr
		},
	}
}

// stubbedCleanupCommand wires a Command whose application invocation resolves a
// GitHub-backed record and whose runners are replaced by configure.
func stubbedCleanupCommand(configure func(*cleanupapp.Invocation, *CleanupDeps)) Command {
	invocation := cleanupapp.Invocation{
		Read: func(id string) (issueopscontract.IssueOpsRecord, error) {
			return issueopscontract.IssueOpsRecord{ID: id, IssueURL: "https://github.com/acme/repo/issues/1"}, nil
		},
		Provider:                   func(string) (port.IssueProvider, error) { return nil, nil },
		CurrentDirectory:           func() (string, error) { return "/cwd", nil },
		MergeVerificationAvailable: true,
	}
	operations := CleanupDeps{
		IssueOpsStateRoot:            func() string { return "/state" },
		ObserveNativeProcessAncestry: func(int) ([]issueopscontract.NativeProcessReceipt, error) { return nil, nil },
	}
	configure(&invocation, &operations)
	return Command{Operations: operations, Invoke: func(Deps) cleanupapp.Invocation { return invocation }}
}

func TestRunCleanupSubcommandsRejectUnknownFlagsAndHonorHelpWithoutRunning(t *testing.T) {
	ran := false
	command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, ops *CleanupDeps) {
		inv.Read = func(string) (issueopscontract.IssueOpsRecord, error) {
			ran = true
			return issueopscontract.IssueOpsRecord{}, nil
		}
		inv.RunAbandon = func(context.Context, issueopscontract.CleanupAbandonRequest) (issueopscontract.CleanupAbandonResult, error) {
			ran = true
			return issueopscontract.CleanupAbandonResult{}, nil
		}
		ops.Status = func(context.Context, string, string, bool, Deps) (issueopscontract.IssueOpsCleanupStatus, error) {
			ran = true
			return issueopscontract.IssueOpsCleanupStatus{}, nil
		}
		ops.CloseIssueOpsChildren = func(string, string, issueopscontract.IssueOpsCloseChildrenRequest, Deps) (issueopscontract.IssueOpsCloseChildrenResult, error) {
			ran = true
			return issueopscontract.IssueOpsCloseChildrenResult{}, nil
		}
		ops.AddIssueOpsFeedbackWithActor = func(string, string, string, string, string, issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			ran = true
			return issueopscontract.IssueOpsRecord{}, nil
		}
		ops.MarkIssueOpsContractFeedbackIssueUpdatedWithActor = func(string, string, issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			ran = true
			return issueopscontract.IssueOpsRecord{}, nil
		}
	})
	outputs := &cleanupOutputs{}
	deps := outputs.deps()
	deps.PrintResult = func(issueopscontract.IssueOpsRecord, bool, error) error { ran = true; return nil }
	deps.OrphanPreview = func(context.Context, orphancontract.Request) (orphancontract.Result, error) {
		ran = true
		return orphancontract.Result{}, nil
	}
	for _, args := range [][]string{
		{"status"}, {"close-children"}, {"orphan"}, {"remote-branch"}, {"linked-branch"}, {"finish"}, {"abandon"},
	} {
		name := args[0]
		t.Run(name+" unknown flag", func(t *testing.T) {
			if err := command.RunCleanup(append(slicesClone(args), "--bogus"), deps); err == nil {
				t.Fatal("unknown flag accepted")
			}
		})
		t.Run(name+" help", func(t *testing.T) {
			if err := command.RunCleanup(append(slicesClone(args), "-h"), deps); err != nil {
				t.Fatalf("help returned error: %v", err)
			}
		})
	}
	for _, sub := range []string{"add", "mark-issue-updated"} {
		t.Run("feedback "+sub, func(t *testing.T) {
			if err := command.RunFeedback([]string{sub, "--bogus"}, deps); err == nil {
				t.Fatal("unknown flag accepted")
			}
			if err := command.RunFeedback([]string{sub, "-h"}, deps); err != nil {
				t.Fatalf("help returned error: %v", err)
			}
		})
	}
	if ran {
		t.Fatal("a rejected or help invocation reached an operation")
	}
}

func slicesClone(values []string) []string { return append([]string(nil), values...) }

func TestRunHelpRendersUsageForEveryEntryAlias(t *testing.T) {
	command := stubbedCleanupCommand(func(*cleanupapp.Invocation, *CleanupDeps) {})
	for _, alias := range []string{"--help", "-h", "help"} {
		out, err := captureCleanupStdout(t, func() error { return command.RunCleanup([]string{alias}, Deps{}) })
		if err != nil || !strings.Contains(out, "Usage: issueops cleanup status") || !strings.Contains(out, "issueops cleanup abandon") {
			t.Fatalf("cleanup %s: out=%q err=%v", alias, out, err)
		}
		out, err = captureCleanupStdout(t, func() error { return command.RunFeedback([]string{alias}, Deps{}) })
		if err != nil || !strings.Contains(out, "Usage: issueops feedback add") || !strings.Contains(out, "mark-issue-updated") {
			t.Fatalf("feedback %s: out=%q err=%v", alias, out, err)
		}
	}
}

func TestRunFeedbackBindsFlagsToTheLocalActorWithObservedAncestry(t *testing.T) {
	ancestry := []issueopscontract.NativeProcessReceipt{{PID: 7, StartedAt: "t", Executable: "codex"}}
	var added, marked issueopscontract.IssueOpsActor
	var addArgs []string
	command := stubbedCleanupCommand(func(_ *cleanupapp.Invocation, ops *CleanupDeps) {
		ops.ObserveNativeProcessAncestry = func(pid int) ([]issueopscontract.NativeProcessReceipt, error) {
			if pid != os.Getpid() {
				t.Errorf("ancestry observed for pid %d, want this process", pid)
			}
			return ancestry, errors.New("partial ancestry is tolerated")
		}
		ops.AddIssueOpsFeedbackWithActor = func(root, id, source, body, classification string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			addArgs = []string{root, id, source, body, classification}
			added = actor
			return issueopscontract.IssueOpsRecord{ID: id}, nil
		}
		ops.MarkIssueOpsContractFeedbackIssueUpdatedWithActor = func(_ string, id string, actor issueopscontract.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
			marked = actor
			return issueopscontract.IssueOpsRecord{ID: id}, nil
		}
	})
	var printed []bool
	deps := Deps{ParseFlags: parseFeedbackCleanupFlags, PrintResult: func(_ issueopscontract.IssueOpsRecord, jsonOut bool, err error) error {
		printed = append(printed, jsonOut)
		return err
	}}
	if err := command.RunFeedback([]string{"add", "--id", "io-1", "--source", "review", "--body", "fix", "--classification", "defect", "--host", "codex", "--session-id", "s1", "--agent-id", "a1", "--cwd", "/work", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	if err := command.RunFeedback([]string{"mark-issue-updated", "--id", "io-1", "--host", "claude", "--session-id", "s2", "--cwd", "/work2"}, deps); err != nil {
		t.Fatal(err)
	}
	wantAdded := issueopscontract.IssueOpsActor{Host: "codex", SessionID: "s1", AgentID: "a1", CWD: "/work", NativeProcessAncestry: ancestry}
	wantMarked := issueopscontract.IssueOpsActor{Host: "claude", SessionID: "s2", CWD: "/work2", NativeProcessAncestry: ancestry}
	if !reflect.DeepEqual(added, wantAdded) || !reflect.DeepEqual(marked, wantMarked) {
		t.Fatalf("added=%+v marked=%+v", added, marked)
	}
	if !reflect.DeepEqual(addArgs, []string{"/state", "io-1", "review", "fix", "defect"}) || !reflect.DeepEqual(printed, []bool{true, false}) {
		t.Fatalf("addArgs=%v printed json flags=%v", addArgs, printed)
	}
}

func TestRunCleanupStatusRendersReadinessAndForwardsFlags(t *testing.T) {
	var gotRoot, gotID string
	var gotMerged bool
	status := issueopscontract.IssueOpsCleanupStatus{}
	var statusErr error
	command := stubbedCleanupCommand(func(_ *cleanupapp.Invocation, ops *CleanupDeps) {
		ops.Status = func(_ context.Context, root, id string, merged bool, _ Deps) (issueopscontract.IssueOpsCleanupStatus, error) {
			gotRoot, gotID, gotMerged = root, id, merged
			return status, statusErr
		}
	})
	outputs := &cleanupOutputs{}

	status = issueopscontract.IssueOpsCleanupStatus{Ready: false, Missing: []string{"pr_phase", "merged"}, Choices: []string{"1) keep", "2) abandon"}}
	out, err := captureCleanupStdout(t, func() error {
		return command.RunCleanup([]string{"status", "--id", "io-9", "--merged"}, outputs.deps())
	})
	want := "ready: false\n- missing: pr_phase\n- missing: merged\n선택지:\n1) keep\n2) abandon\n"
	if err != nil || out != want {
		t.Fatalf("out=%q want %q err=%v", out, want, err)
	}
	if gotRoot != "/state" || gotID != "io-9" || !gotMerged || len(outputs.json) != 0 {
		t.Fatalf("root=%q id=%q merged=%v json=%v", gotRoot, gotID, gotMerged, outputs.json)
	}

	status = issueopscontract.IssueOpsCleanupStatus{Ready: true}
	out, err = captureCleanupStdout(t, func() error { return command.RunCleanup([]string{"status", "--id", "io-9"}, outputs.deps()) })
	if err != nil || out != "ready: true\n" || gotMerged {
		t.Fatalf("ready out=%q merged=%v err=%v", out, gotMerged, err)
	}

	// A failed status prints nothing in text mode and returns the cause as is.
	statusErr = errors.New("record missing")
	out, err = captureCleanupStdout(t, func() error { return command.RunCleanup([]string{"status", "--id", "io-9"}, outputs.deps()) })
	if err != statusErr || out != "" || len(outputs.errors) != 0 {
		t.Fatalf("text failure out=%q err=%v printed=%v", out, err, outputs.errors)
	}

	// JSON error rendering failure outranks the original cause.
	renderErr := errors.New("stdout closed")
	outputs.errErr = renderErr
	if err := command.RunCleanup([]string{"status", "--id", "io-9", "--json"}, outputs.deps()); err != renderErr {
		t.Fatalf("render failure must win, got %v", err)
	}
}

func TestRunCleanupCloseChildrenRendersPreviewAndClosedChildren(t *testing.T) {
	result := issueopscontract.IssueOpsCloseChildrenResult{
		ClosedCount: 1,
		Children: []issueopscontract.IssueOpsCloseChildResult{
			{URL: "https://example.test/c/1", Preview: "would close https://example.test/c/1"},
			{URL: "https://example.test/c/2", Closed: true, State: "closed"},
		},
	}
	var runErr error
	command := stubbedCleanupCommand(func(_ *cleanupapp.Invocation, ops *CleanupDeps) {
		ops.CloseIssueOpsChildren = func(string, string, issueopscontract.IssueOpsCloseChildrenRequest, Deps) (issueopscontract.IssueOpsCloseChildrenResult, error) {
			return result, runErr
		}
	})
	outputs := &cleanupOutputs{}
	out, err := captureCleanupStdout(t, func() error {
		return command.RunCleanup([]string{"close-children", "--id", "io-9", "--merged"}, outputs.deps())
	})
	want := "closed children: 1\nwould close https://example.test/c/1\n- https://example.test/c/2 closed=true state=closed\n"
	if err != nil || out != want {
		t.Fatalf("out=%q want %q err=%v", out, want, err)
	}

	runErr = errors.New("provider refused")
	out, err = captureCleanupStdout(t, func() error {
		return command.RunCleanup([]string{"close-children", "--id", "io-9"}, outputs.deps())
	})
	if err != runErr || out != "" || len(outputs.errors) != 0 {
		t.Fatalf("text failure out=%q err=%v printed=%v", out, err, outputs.errors)
	}
}

func TestRunOrphanCleanupRejectsInconsistentModeFlagsBeforeObserving(t *testing.T) {
	calls := 0
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		OrphanPreview: func(context.Context, orphancontract.Request) (orphancontract.Result, error) {
			calls++
			return orphancontract.Result{}, nil
		},
		OrphanApply: func(context.Context, orphancontract.Request, orphancontract.ApplyRequest) (orphancontract.Result, error) {
			calls++
			return orphancontract.Result{}, nil
		},
	}
	for name, tt := range map[string]struct {
		args []string
		want string
	}{
		"confirm without apply":     {[]string{"orphan", "--confirm"}, "--confirm requires --apply"},
		"fingerprint without apply": {[]string{"orphan", "--fingerprint", "abc"}, "--fingerprint requires --apply"},
		"apply without confirm":     {[]string{"orphan", "--apply", "--fingerprint", "abc"}, "apply requires --confirm"},
	} {
		t.Run(name, func(t *testing.T) {
			err := (Command{}).RunCleanup(tt.args, deps)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error=%v want %q", err, tt.want)
			}
		})
	}
	if calls != 0 {
		t.Fatalf("operation reached %d times for a rejected invocation", calls)
	}
	// A blank fingerprint is not a fingerprint: it must not demand --apply.
	if err := (Command{}).RunCleanup([]string{"orphan", "--fingerprint", "   "}, deps); err != nil || calls != 1 {
		t.Fatalf("blank fingerprint err=%v calls=%d", err, calls)
	}
}

func TestRunOrphanCleanupFailsClosedWhenOperationIsNotWired(t *testing.T) {
	base := Deps{ParseFlags: parseFeedbackCleanupFlags}
	if err := (Command{}).RunCleanup([]string{"orphan"}, base); err == nil || !strings.Contains(err.Error(), "preview is unavailable") {
		t.Fatalf("preview without wiring: %v", err)
	}
	if err := (Command{}).RunCleanup([]string{"orphan", "--apply", "--confirm"}, base); err == nil || !strings.Contains(err.Error(), "apply is unavailable") {
		t.Fatalf("apply without wiring: %v", err)
	}
}

func TestRunOrphanCleanupRendersResultAndStillReturnsTheOperationError(t *testing.T) {
	result := orphancontract.Result{
		Ready: false, HeadSHA: "abc123", RecoveryPath: "/recovery",
		Missing: []string{"target_clean"}, Warnings: []string{"remote unreachable"},
		Fingerprint: "fp-1", RemoteBranchDeletion: "kept",
	}
	opErr := errors.New("preview refused")
	var request orphancontract.Request
	var apply orphancontract.ApplyRequest
	deps := Deps{
		ParseFlags: parseFeedbackCleanupFlags,
		OrphanPreview: func(_ context.Context, req orphancontract.Request) (orphancontract.Result, error) {
			request = req
			return result, opErr
		},
		OrphanApply: func(_ context.Context, req orphancontract.Request, applyReq orphancontract.ApplyRequest) (orphancontract.Result, error) {
			request, apply = req, applyReq
			return result, nil
		},
	}
	args := []string{"orphan", "--id", "io-gone", "--repo", "/repo", "--worktree", "/wt", "--branch", "b", "--provider", "github", "--kind", "pr", "--artifact-url", "https://github.com/acme/repo/pull/1"}
	out, err := captureCleanupStdout(t, func() error { return (Command{}).RunCleanup(args, deps) })
	want := "ready: false\nhead: abc123\nrecovery path: /recovery\n- missing: target_clean\n- warning: remote unreachable\nfingerprint: fp-1\nremote branch: kept\n"
	if err != opErr || out != want {
		t.Fatalf("out=%q want %q err=%v", out, want, err)
	}
	if request.ID != "io-gone" || request.RepoRoot != "/repo" || request.WorktreePath != "/wt" || request.Branch != "b" ||
		request.Artifact.Provider != "github" || request.Artifact.Kind != "pr" || request.Artifact.URL != "https://github.com/acme/repo/pull/1" {
		t.Fatalf("request=%+v", request)
	}

	// Optional lines stay out of a minimal result.
	deps.OrphanPreview = func(context.Context, orphancontract.Request) (orphancontract.Result, error) {
		return orphancontract.Result{Ready: true, HeadSHA: "h"}, nil
	}
	out, err = captureCleanupStdout(t, func() error { return (Command{}).RunCleanup([]string{"orphan"}, deps) })
	if err != nil || out != "ready: true\nhead: h\nrecovery path: \n" {
		t.Fatalf("minimal out=%q err=%v", out, err)
	}

	// JSON mode emits the result before surfacing the error; a failed render outranks it.
	outputs := &cleanupOutputs{}
	jsonDeps := outputs.deps()
	jsonDeps.OrphanPreview = deps.OrphanPreview
	jsonDeps.OrphanApply = deps.OrphanApply
	if err := (Command{}).RunCleanup(append(slicesClone(args), "--json", "--apply", "--confirm", "--fingerprint", "fp-1"), jsonDeps); err != nil {
		t.Fatalf("json apply: %v", err)
	}
	if len(outputs.json) != 1 || outputs.json[0].(orphancontract.Result).Fingerprint != "fp-1" || apply != (orphancontract.ApplyRequest{Confirm: true, Fingerprint: "fp-1"}) {
		t.Fatalf("json=%v apply=%+v", outputs.json, apply)
	}
	outputs.jsonErr = errors.New("stdout closed")
	if err := (Command{}).RunCleanup(append(slicesClone(args), "--json", "--apply", "--confirm"), jsonDeps); err != outputs.jsonErr {
		t.Fatalf("render failure must win, got %v", err)
	}
}

func TestRunCleanupFinishRejectsAmbiguousOrMissingMode(t *testing.T) {
	command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
		inv.RunFinish = func(context.Context, issueopscontract.CleanupFinishRequest, port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			t.Fatal("finish ran for an invalid mode")
			return issueopscontract.CleanupFinishResult{}, nil
		}
	})
	deps := (&cleanupOutputs{}).deps()
	if err := command.RunCleanup([]string{"finish", "--id", "io-1", "--preview", "--apply"}, deps); err == nil || !strings.Contains(err.Error(), "--preview and --apply are mutually exclusive") {
		t.Fatalf("both modes: %v", err)
	}
	if err := command.RunCleanup([]string{"finish", "--id", "io-1"}, deps); err == nil || !strings.Contains(err.Error(), "requires exactly one mode") {
		t.Fatalf("no mode: %v", err)
	}
}

func TestRunCleanupFinishForwardsRequestAndProviderOverride(t *testing.T) {
	var request issueopscontract.CleanupFinishRequest
	var providerName string
	command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
		inv.Provider = func(name string) (port.IssueProvider, error) { providerName = name; return nil, nil }
		inv.RunFinish = func(_ context.Context, req issueopscontract.CleanupFinishRequest, _ port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			request = req
			return issueopscontract.CleanupFinishResult{ID: req.ID, Fingerprint: "fp"}, nil
		}
	})
	outputs := &cleanupOutputs{}
	_, err := captureCleanupStdout(t, func() error {
		return command.RunCleanup([]string{"finish", "--id", "io-1", "--provider", "gitlab", "--apply", "--confirm", "--fingerprint", "fp-9", "--superseded-by", "  https://gitlab.test/mr/2  ", "--keep-remote-branch"}, outputs.deps())
	})
	if err != nil {
		t.Fatal(err)
	}
	want := issueopscontract.CleanupFinishRequest{ID: "io-1", CWD: "/cwd", Apply: true, Confirm: true, Fingerprint: "fp-9", SupersededBy: "https://gitlab.test/mr/2", KeepRemoteBranch: true}
	if request != want || providerName != "gitlab" {
		t.Fatalf("request=%+v provider=%q", request, providerName)
	}
}

func TestRunCleanupFinishRendersOutcome(t *testing.T) {
	tests := []struct {
		name   string
		result issueopscontract.CleanupFinishResult
		want   string
	}{
		{"record deleted", issueopscontract.CleanupFinishResult{RecordDeleted: true, WorktreeRemoved: true, BranchDeleted: true}, "cleanup finished: worktree=true branch=true record deleted\n"},
		{"record deleted keeping remote branch", issueopscontract.CleanupFinishResult{RecordDeleted: true, WorktreeRemoved: true, KeptRemoteBranch: &issueopscontract.CleanupKeptRemoteBranch{Branch: "feat/x", State: issueopscontract.CleanupKeptRemoteBranchPresent, RemoteOID: "abc"}}, "cleanup finished: worktree=true branch=false record deleted\nremote branch kept: branch=feat/x state=present oid=abc\n"},
		{"preview with next command", issueopscontract.CleanupFinishResult{Fingerprint: "fp-1", NextCommand: "issueops cleanup finish --apply"}, "fingerprint: fp-1\nnext: issueops cleanup finish --apply\n"},
		{"preview without next command", issueopscontract.CleanupFinishResult{Fingerprint: "fp-1"}, "fingerprint: fp-1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
				inv.RunFinish = func(context.Context, issueopscontract.CleanupFinishRequest, port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
					return tt.result, nil
				}
			})
			outputs := &cleanupOutputs{}
			out, err := captureCleanupStdout(t, func() error {
				return command.RunCleanup([]string{"finish", "--id", "io-1", "--preview"}, outputs.deps())
			})
			if err != nil || out != tt.want || len(outputs.json) != 0 {
				t.Fatalf("out=%q want %q err=%v json=%v", out, tt.want, err, outputs.json)
			}
		})
	}
}

func TestRunCleanupFinishErrorAndJSONContracts(t *testing.T) {
	gateErr := errors.New("gates failed")
	result := issueopscontract.CleanupFinishResult{ID: "io-1", Missing: []string{"merged"}}
	command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
		inv.RunFinish = func(context.Context, issueopscontract.CleanupFinishRequest, port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			return result, gateErr
		}
	})

	// JSON: the gate result is the audit channel and precedes the error.
	outputs := &cleanupOutputs{}
	if err := command.RunCleanup([]string{"finish", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != gateErr {
		t.Fatalf("error=%v", err)
	}
	if len(outputs.json) != 1 || !reflect.DeepEqual(outputs.json[0], result) || len(outputs.errors) != 0 {
		t.Fatalf("json=%v errors=%v", outputs.json, outputs.errors)
	}
	// Text: the error alone is returned and nothing is emitted as JSON.
	outputs = &cleanupOutputs{}
	if err := command.RunCleanup([]string{"finish", "--id", "io-1", "--preview"}, outputs.deps()); err != gateErr || len(outputs.json) != 0 {
		t.Fatalf("error=%v json=%v", err, outputs.json)
	}
	// A failed JSON render outranks the gate error.
	outputs = &cleanupOutputs{jsonErr: errors.New("stdout closed")}
	if err := command.RunCleanup([]string{"finish", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != outputs.jsonErr {
		t.Fatalf("render failure must win, got %v", err)
	}

	// Success with --json hands the result to PrintJSON and returns its error.
	ok := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
		inv.RunFinish = func(context.Context, issueopscontract.CleanupFinishRequest, port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
			return issueopscontract.CleanupFinishResult{OK: true, ID: "io-1"}, nil
		}
	})
	outputs = &cleanupOutputs{jsonErr: errors.New("stdout closed")}
	if err := ok.RunCleanup([]string{"finish", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != outputs.jsonErr || len(outputs.json) != 1 {
		t.Fatalf("err=%v json=%v", err, outputs.json)
	}
}

func TestRunCleanupInvocationFailuresRenderOnlyTheOriginalCause(t *testing.T) {
	readErr := errors.New("record store offline")
	tests := []struct {
		name    string
		args    []string
		prepare func(*cleanupapp.Invocation)
		want    string
	}{
		{"finish cannot read record", []string{"finish", "--id", "io-1", "--preview"}, func(inv *cleanupapp.Invocation) {
			inv.Read = func(string) (issueopscontract.IssueOpsRecord, error) {
				return issueopscontract.IssueOpsRecord{}, readErr
			}
		}, "record store offline"},
		{"finish cannot determine provider", []string{"finish", "--id", "io-1", "--preview"}, func(inv *cleanupapp.Invocation) {
			inv.Read = func(id string) (issueopscontract.IssueOpsRecord, error) {
				return issueopscontract.IssueOpsRecord{ID: id}, nil
			}
		}, "pass --provider"},
		{"remote-branch without merge verification", []string{"remote-branch", "--id", "io-1", "--preview"}, func(inv *cleanupapp.Invocation) {
			inv.MergeVerificationAvailable = false
		}, "merge verification is not configured"},
		{"linked-branch cannot read record", []string{"linked-branch", "--id", "io-1", "--preview"}, func(inv *cleanupapp.Invocation) {
			inv.Read = func(string) (issueopscontract.IssueOpsRecord, error) {
				return issueopscontract.IssueOpsRecord{}, readErr
			}
		}, "record store offline"},
		{"abandon cannot bind next command", []string{"abandon", "--id", "io-1", "--reason", "r", "--preview"}, func(inv *cleanupapp.Invocation) {
			inv.RunAbandon = func(context.Context, issueopscontract.CleanupAbandonRequest) (issueopscontract.CleanupAbandonResult, error) {
				return issueopscontract.CleanupAbandonResult{NextCommand: "issueops cleanup abandon --apply"}, nil
			}
			inv.Read = func(string) (issueopscontract.IssueOpsRecord, error) {
				return issueopscontract.IssueOpsRecord{}, readErr
			}
		}, "record store offline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
				inv.RunFinish = func(context.Context, issueopscontract.CleanupFinishRequest, port.IssueProvider) (issueopscontract.CleanupFinishResult, error) {
					t.Fatal("runner reached after an invocation failure")
					return issueopscontract.CleanupFinishResult{}, nil
				}
				inv.RunRemoteBranch = func(context.Context, issueopscontract.CleanupRemoteBranchRequest, port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
					t.Fatal("runner reached after an invocation failure")
					return issueopscontract.CleanupRemoteBranchResult{}, nil
				}
				inv.RunLinkedBranch = func(context.Context, issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
					t.Fatal("runner reached after an invocation failure")
					return issueopscontract.CleanupLinkedBranchResult{}, nil
				}
				tt.prepare(inv)
			})
			// Text mode returns the cause and emits no JSON.
			outputs := &cleanupOutputs{}
			err := command.RunCleanup(tt.args, outputs.deps())
			if err == nil || !strings.Contains(err.Error(), tt.want) || len(outputs.json) != 0 || len(outputs.errors) != 0 {
				t.Fatalf("text err=%v json=%v errors=%v", err, outputs.json, outputs.errors)
			}
			// JSON mode renders exactly the unwrapped cause through PrintError.
			outputs = &cleanupOutputs{}
			err = command.RunCleanup(append(slicesClone(tt.args), "--json"), outputs.deps())
			var wrapped *port.CleanupInvocationError
			if err == nil || errors.As(err, &wrapped) || len(outputs.errors) != 1 || outputs.errors[0] != err || len(outputs.json) != 0 {
				t.Fatalf("json err=%v rendered=%v json=%v", err, outputs.errors, outputs.json)
			}
			// A failing renderer outranks the cause.
			outputs = &cleanupOutputs{errErr: errors.New("stdout closed")}
			if err := command.RunCleanup(append(slicesClone(tt.args), "--json"), outputs.deps()); err != outputs.errErr {
				t.Fatalf("render failure must win, got %v", err)
			}
		})
	}
}

func TestRunCleanupRemoteBranchRendersOutcomeAndJSONContract(t *testing.T) {
	tests := []struct {
		name   string
		result issueopscontract.CleanupRemoteBranchResult
		want   string
	}{
		{"deleted", issueopscontract.CleanupRemoteBranchResult{Deleted: true, Branch: "feat/x", RemoteOID: "abc", DeletedAt: "2026-10-12T00:00:00Z"}, "remote branch deleted: branch=feat/x oid=abc at=2026-10-12T00:00:00Z\n"},
		{"already absent", issueopscontract.CleanupRemoteBranchResult{AlreadyAbsent: true, Branch: "feat/x"}, "remote branch already absent: branch=feat/x\n"},
		{"preview", issueopscontract.CleanupRemoteBranchResult{Fingerprint: "fp-2", NextCommand: "issueops cleanup remote-branch --apply"}, "fingerprint: fp-2\nnext: issueops cleanup remote-branch --apply\n"},
		{"preview without next", issueopscontract.CleanupRemoteBranchResult{Fingerprint: "fp-2"}, "fingerprint: fp-2\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runErr error
			command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
				inv.RunRemoteBranch = func(context.Context, issueopscontract.CleanupRemoteBranchRequest, port.IssueProvider) (issueopscontract.CleanupRemoteBranchResult, error) {
					return tt.result, runErr
				}
			})
			outputs := &cleanupOutputs{}
			out, err := captureCleanupStdout(t, func() error {
				return command.RunCleanup([]string{"remote-branch", "--id", "io-1", "--preview"}, outputs.deps())
			})
			if err != nil || out != tt.want || len(outputs.json) != 0 {
				t.Fatalf("out=%q want %q err=%v", out, tt.want, err)
			}
			// JSON success hands the result over untouched.
			outputs = &cleanupOutputs{}
			if err := command.RunCleanup([]string{"remote-branch", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != nil || len(outputs.json) != 1 || !reflect.DeepEqual(outputs.json[0], tt.result) {
				t.Fatalf("json err=%v json=%v", err, outputs.json)
			}
			// A runner error keeps the result in JSON and returns the error alone in text.
			runErr = errors.New("remote delete refused")
			outputs = &cleanupOutputs{}
			if err := command.RunCleanup([]string{"remote-branch", "--id", "io-1", "--apply", "--confirm", "--fingerprint", "fp", "--json"}, outputs.deps()); err != runErr || len(outputs.json) != 1 {
				t.Fatalf("json failure err=%v json=%v", err, outputs.json)
			}
			outputs = &cleanupOutputs{}
			out, err = captureCleanupStdout(t, func() error {
				return command.RunCleanup([]string{"remote-branch", "--id", "io-1", "--apply", "--confirm", "--fingerprint", "fp"}, outputs.deps())
			})
			if err != runErr || out != "" || len(outputs.json) != 0 {
				t.Fatalf("text failure out=%q err=%v json=%v", out, err, outputs.json)
			}
			outputs = &cleanupOutputs{jsonErr: errors.New("stdout closed")}
			if err := command.RunCleanup([]string{"remote-branch", "--id", "io-1", "--apply", "--confirm", "--fingerprint", "fp", "--json"}, outputs.deps()); err != outputs.jsonErr {
				t.Fatalf("render failure must win, got %v", err)
			}
			runErr = nil
		})
	}
}

func TestRunCleanupAbandonRendersOutcomeAndRemoteEffects(t *testing.T) {
	tests := []struct {
		name   string
		result issueopscontract.CleanupAbandonResult
		want   string
	}{
		{
			name: "applied with every remote effect",
			result: issueopscontract.CleanupAbandonResult{
				RecordDeleted: true, WorktreeRemoved: true, BranchDeleted: true, IntentRowsDeleted: []string{"a", "b"}, AbandonedAt: "2026-10-12T00:00:00Z",
				RemoteEffects: []string{"close_pr", "close_issue"}, RemoteArtifactState: "open", IssueState: "open", RemoteBranchOID: "deadbeef",
			},
			want: "cleanup abandoned: worktree_removed=true branch_deleted=true record_deleted=true intent_rows=2 at=2026-10-12T00:00:00Z\n" +
				"remote effects: close_pr, close_issue\n  artifact state: open\n  issue state: open\n  remote branch oid: deadbeef\n",
		},
		{
			name:   "applied without remote effects",
			result: issueopscontract.CleanupAbandonResult{RecordDeleted: true, AbandonedAt: "t"},
			want:   "cleanup abandoned: worktree_removed=false branch_deleted=false record_deleted=true intent_rows=0 at=t\n",
		},
		{
			name:   "preview lists effects before the next command",
			result: issueopscontract.CleanupAbandonResult{Fingerprint: "fp-3", RemoteEffects: []string{"close_issue"}, IssueState: "open", NextCommand: "issueops cleanup abandon --apply"},
			want:   "fingerprint: fp-3\nremote effects: close_issue\n  issue state: open\nnext: issueops cleanup abandon --apply\n",
		},
		{
			name:   "preview with only the artifact observed",
			result: issueopscontract.CleanupAbandonResult{Fingerprint: "fp-3", RemoteEffects: []string{"close_pr"}, RemoteArtifactState: "open"},
			want:   "fingerprint: fp-3\nremote effects: close_pr\n  artifact state: open\n",
		},
		{
			name:   "preview without effects or next command",
			result: issueopscontract.CleanupAbandonResult{Fingerprint: "fp-3"},
			want:   "fingerprint: fp-3\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
				inv.RunAbandon = func(context.Context, issueopscontract.CleanupAbandonRequest) (issueopscontract.CleanupAbandonResult, error) {
					return tt.result, nil
				}
			})
			outputs := &cleanupOutputs{}
			out, err := captureCleanupStdout(t, func() error {
				return command.RunCleanup([]string{"abandon", "--id", "io-1", "--reason", "obsolete", "--preview"}, outputs.deps())
			})
			if err != nil || out != tt.want || len(outputs.json) != 0 {
				t.Fatalf("out=%q want %q err=%v", out, tt.want, err)
			}
		})
	}
}

func TestRunCleanupAbandonErrorKeepsResultAsTheAuditChannelInJSON(t *testing.T) {
	gateErr := errors.New("abandon gates failed")
	result := issueopscontract.CleanupAbandonResult{ID: "io-1", Missing: []string{"no_children"}, UnresolvedChildren: []string{"https://example.test/c/1"}}
	command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
		inv.RunAbandon = func(context.Context, issueopscontract.CleanupAbandonRequest) (issueopscontract.CleanupAbandonResult, error) {
			return result, gateErr
		}
	})
	outputs := &cleanupOutputs{}
	if err := command.RunCleanup([]string{"abandon", "--id", "io-1", "--reason", "r", "--preview", "--json"}, outputs.deps()); err != gateErr {
		t.Fatalf("error=%v", err)
	}
	if len(outputs.json) != 1 || !reflect.DeepEqual(outputs.json[0], result) {
		t.Fatalf("json=%v", outputs.json)
	}
	outputs = &cleanupOutputs{}
	out, err := captureCleanupStdout(t, func() error {
		return command.RunCleanup([]string{"abandon", "--id", "io-1", "--reason", "r", "--preview"}, outputs.deps())
	})
	if err != gateErr || out != "" || len(outputs.json) != 0 {
		t.Fatalf("text out=%q err=%v json=%v", out, err, outputs.json)
	}
	outputs = &cleanupOutputs{jsonErr: errors.New("stdout closed")}
	if err := command.RunCleanup([]string{"abandon", "--id", "io-1", "--reason", "r", "--preview", "--json"}, outputs.deps()); err != outputs.jsonErr {
		t.Fatalf("render failure must win, got %v", err)
	}
	outputs = &cleanupOutputs{jsonErr: errors.New("stdout closed")}
	ok := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
		inv.RunAbandon = func(context.Context, issueopscontract.CleanupAbandonRequest) (issueopscontract.CleanupAbandonResult, error) {
			return result, nil
		}
	})
	if err := ok.RunCleanup([]string{"abandon", "--id", "io-1", "--reason", "r", "--preview", "--json"}, outputs.deps()); err != outputs.jsonErr || len(outputs.json) != 1 {
		t.Fatalf("success render failure err=%v json=%v", err, outputs.json)
	}
}

func TestRunCleanupLinkedBranchRendersEachClassification(t *testing.T) {
	tests := []struct {
		name   string
		result issueopscontract.CleanupLinkedBranchResult
		want   string
	}{
		{"deleted", issueopscontract.CleanupLinkedBranchResult{Deleted: true, LinkedBranchID: "LB_1", DeletedAt: "2026-10-12T00:00:00Z"}, "orphan linked branch deleted: node=LB_1 at=2026-10-12T00:00:00Z\n"},
		{"already absent", issueopscontract.CleanupLinkedBranchResult{AlreadyAbsent: true, IssueURL: "https://github.com/acme/repo/issues/1", RequestedBranch: "1-x"}, "linked branch already absent: issue=https://github.com/acme/repo/issues/1 branch=1-x\n"},
		{"orphan preview", issueopscontract.CleanupLinkedBranchResult{State: "orphan_ref_null", StateReason: "ref is null", LinkedBranchID: "LB_1", NextCommand: "issueops cleanup linked-branch --apply"}, "state: orphan_ref_null\nreason: ref is null\ntarget: LB_1\nnext: issueops cleanup linked-branch --apply\n"},
		{"ambiguous preview names no target", issueopscontract.CleanupLinkedBranchResult{State: "ambiguous"}, "state: ambiguous\n"},
		{"audit failure is surfaced after a deletion", issueopscontract.CleanupLinkedBranchResult{Deleted: true, LinkedBranchID: "LB_1", DeletedAt: "t", AuditError: "audit write failed"}, "orphan linked branch deleted: node=LB_1 at=t\naudit error: audit write failed\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runErr error
			command := stubbedCleanupCommand(func(inv *cleanupapp.Invocation, _ *CleanupDeps) {
				inv.RunLinkedBranch = func(context.Context, issueopscontract.CleanupLinkedBranchRequest) (issueopscontract.CleanupLinkedBranchResult, error) {
					return tt.result, runErr
				}
			})
			outputs := &cleanupOutputs{}
			out, err := captureCleanupStdout(t, func() error {
				return command.RunCleanup([]string{"linked-branch", "--id", "io-1", "--preview"}, outputs.deps())
			})
			if err != nil || out != tt.want || len(outputs.json) != 0 {
				t.Fatalf("out=%q want %q err=%v", out, tt.want, err)
			}
			outputs = &cleanupOutputs{}
			if err := command.RunCleanup([]string{"linked-branch", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != nil || len(outputs.json) != 1 || !reflect.DeepEqual(outputs.json[0], tt.result) {
				t.Fatalf("json err=%v json=%v", err, outputs.json)
			}
			runErr = errors.New("graphql refused")
			outputs = &cleanupOutputs{}
			if err := command.RunCleanup([]string{"linked-branch", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != runErr || len(outputs.json) != 1 {
				t.Fatalf("json failure err=%v json=%v", err, outputs.json)
			}
			out, err = captureCleanupStdout(t, func() error {
				return command.RunCleanup([]string{"linked-branch", "--id", "io-1", "--preview"}, (&cleanupOutputs{}).deps())
			})
			if err != runErr || out != "" {
				t.Fatalf("text failure out=%q err=%v", out, err)
			}
			outputs = &cleanupOutputs{jsonErr: errors.New("stdout closed")}
			if err := command.RunCleanup([]string{"linked-branch", "--id", "io-1", "--preview", "--json"}, outputs.deps()); err != outputs.jsonErr {
				t.Fatalf("render failure must win, got %v", err)
			}
			runErr = nil
		})
	}
}
