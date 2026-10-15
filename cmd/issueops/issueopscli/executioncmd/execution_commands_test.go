package executioncmd

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	executionissue "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	provenanceport "issueops/internal/port/issueopsprovenance"
	"issueops/internal/testsupport"
)

var explicitActorArgs = []string{
	"--host", "codex", "--session-id", "sess-1", "--session-pid", "42",
	"--session-started-at", "2026-08-01T00:00:00Z", "--session-executable", "/opt/codex", "--cwd", "/work",
}

func fixedAncestry(int) ([]model.NativeProcessReceipt, error) { return nil, nil }

// capturingDeps returns Deps whose ExecuteExecution records the request and whose
// JSON printer records the printed value.
func capturingDeps(got *model.ExecutionActionRequest, printed *[]any, result any, execErr error) Deps {
	return Deps{
		StateRoot: func() string { return "/state" },
		Runtime: ExecutionDeps{
			ObserveNativeProcessAncestry: fixedAncestry,
			ExecuteExecution: func(_ context.Context, root string, req model.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
				if root != "/state" {
					return nil, errors.New("unexpected state root " + root)
				}
				*got = req
				return result, execErr
			},
		},
		PrintJSON: func(v any) error { *printed = append(*printed, v); return nil },
	}
}

func TestRunPrintsUsageForEmptyAndHelpAndRejectsUnknownSubcommand(t *testing.T) {
	for _, args := range [][]string{nil, {"help"}, {"-h"}, {"--help"}} {
		out := testsupport.CaptureStdout(t, func() error { return Run(args, Deps{}) })
		if out != Usage+"\n" {
			t.Fatalf("args=%v out=%q", args, out)
		}
	}
	if err := Run([]string{"bogus"}, Deps{}); err == nil || err.Error() != `unknown issueops execution subcommand "bogus"` {
		t.Fatalf("err=%v", err)
	}
}

func TestSubcommandHelpFlagExitsWithoutExecuting(t *testing.T) {
	calls := 0
	deps := Deps{StateRoot: func() string { return "/s" }, Runtime: ExecutionDeps{ExecuteExecution: func(context.Context, string, model.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error) {
		calls++
		return nil, nil
	}}}
	for _, sub := range []string{"status", "prepare", "release", "complete", "reconcile", "replace", "claim", "resume", "whoami", "sync-base", "switch-mode", "handoff-cmux"} {
		var runErr error
		testsupport.CaptureStderr(t, func() error { runErr = Run([]string{sub, "-h"}, deps); return nil })
		if runErr != nil {
			t.Fatalf("%s -h err=%v", sub, runErr)
		}
	}
	if calls != 0 {
		t.Fatalf("help executed %d times", calls)
	}
}

func TestSubcommandsRejectUnknownFlags(t *testing.T) {
	for _, sub := range []string{"status", "prepare", "release", "complete", "reconcile", "replace", "claim", "resume", "whoami", "sync-base", "switch-mode", "handoff-cmux"} {
		var runErr error
		testsupport.CaptureStderr(t, func() error { runErr = Run([]string{sub, "--no-such-flag"}, Deps{}); return nil })
		if runErr == nil || !strings.Contains(runErr.Error(), "flag provided but not defined") {
			t.Fatalf("%s err=%v", sub, runErr)
		}
	}
}

func TestExecuteFailsClosedWithoutStateRootOrRuntime(t *testing.T) {
	if err := Run([]string{"status", "--id", "io-1"}, Deps{}); err == nil || err.Error() != "IssueOps state root is unavailable" {
		t.Fatalf("no state root: %v", err)
	}
	err := Run([]string{"status", "--id", "io-1"}, Deps{StateRoot: func() string { return "/s" }})
	if err == nil || err.Error() != "issueops execution is not configured" {
		t.Fatalf("no runtime: %v", err)
	}
}

func TestStatusReleaseCompleteReconcileMapFlagsToActionRequests(t *testing.T) {
	var got model.ExecutionActionRequest
	var printed []any
	deps := capturingDeps(&got, &printed, model.ExecutionResult{OK: true, ID: "io-1"}, nil)

	if err := Run([]string{"status", "--id", "io-1", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionStatus || got.ID != "io-1" {
		t.Fatalf("status req=%+v", got)
	}

	args := append([]string{"release", "--id", "io-2", "--generation", "5", "--json"}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionRelease || got.ID != "io-2" || got.Generation != 5 || got.CWD != "/work" ||
		got.Actor.Host != "codex" || got.Actor.SessionID != "sess-1" || got.Actor.SessionProcess == nil || got.Actor.SessionProcess.PID != 42 {
		t.Fatalf("release req=%+v", got)
	}

	args = append([]string{
		"complete", "--id", "io-3", "--generation", "6", "--final-head", "abc123", "--verification-report", "/r.md",
		"--verification", "go test", "--verification", "go vet", "--remote-artifact-url", "https://x/pr/1", "--confirm", "--json",
	}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionComplete || got.ID != "io-3" || got.Generation != 6 || got.FinalHead != "abc123" ||
		got.VerificationReportPath != "/r.md" || !reflect.DeepEqual(got.Verification, []string{"go test", "go vet"}) ||
		got.RemoteArtifactURL != "https://x/pr/1" || !got.Confirm || got.CWD != "/work" {
		t.Fatalf("complete req=%+v", got)
	}

	args = append([]string{"reconcile", "--id", "io-4", "--preview", "--confirm", "--json"}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionReconcile || got.ID != "io-4" || !got.Preview || !got.Confirm || got.IssueSnapshot != nil {
		t.Fatalf("reconcile req=%+v", got)
	}
	if len(printed) != 4 {
		t.Fatalf("printed=%d", len(printed))
	}
}

func TestReconcileAndPrepareAndReplaceRejectUnreadableSnapshotFileBeforeExecuting(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none.json")
	for name, args := range map[string][]string{
		"prepare":   {"prepare", "--id", "io-1", "--issue-snapshot-file", missing},
		"reconcile": {"reconcile", "--id", "io-1", "--issue-snapshot-file", missing},
		"replace":   {"replace", "--id", "io-1", "--preview", "--issue-snapshot-file", missing},
		"claim":     append([]string{"claim", "--id", "io-1", "--claim-current-token", "--issue-snapshot-file", missing}, explicitActorArgs...),
	} {
		t.Run(name, func(t *testing.T) {
			var got model.ExecutionActionRequest
			var printed []any
			errPrinted := 0
			deps := capturingDeps(&got, &printed, nil, nil)
			deps.PrintError = func(error) error { errPrinted++; return nil }
			err := Run(append(args, "--json"), deps)
			if err == nil || !strings.Contains(err.Error(), "inspect issue snapshot file") {
				t.Fatalf("err=%v", err)
			}
			if got.Action != "" || len(printed) != 0 || errPrinted != 1 {
				t.Fatalf("executed=%q printed=%d errPrinted=%d", got.Action, len(printed), errPrinted)
			}
		})
	}
}

func TestPrepareMapsOwnerDirectReasonAndFingerprintFlags(t *testing.T) {
	dir := t.TempDir()
	snapshot := filepath.Join(dir, "s.json")
	if err := os.WriteFile(snapshot, []byte(validExecutionSnapshotJSON()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(snapshot, 0o600); err != nil {
		t.Fatal(err)
	}
	var got model.ExecutionActionRequest
	var printed []any
	deps := capturingDeps(&got, &printed, model.ExecutionPrepareResult{OK: true}, nil)
	args := append([]string{
		"prepare", "--id", "io-1", "--owner-host", "claude", "--owner-model", "m", "--owner-effort", "high",
		"--direct-reason", "no orca", "--expected-readiness-fingerprint", "fp", "--confirm", "--issue-snapshot-file", snapshot, "--json",
	}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	want := model.ExecutionActionRequest{
		Action: model.ExecutionActionPrepare, ID: "io-1", Mode: "auto", CWD: "/work", OwnerHost: "claude", OwnerModel: "m", OwnerEffort: "high",
		IssueSnapshotFile: snapshot, DirectReason: "no orca", ExpectedReadinessFingerprint: "fp", Confirm: true,
		IssueSnapshot: &executionissue.ExecutionIssueSnapshotEvidence{Provider: "gitlab", Source: "glab_mcp", WebURL: "https://gitlab.example.com/acme/repo/-/issues/69", Body: "AC-69", State: "opened"},
	}
	got.Actor = model.NativeActor{}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v\nwant=%+v", got, want)
	}
}

func TestClaimMapsExplicitActorAndDigests(t *testing.T) {
	var got model.ExecutionActionRequest
	var printed []any
	deps := capturingDeps(&got, &printed, model.ExecutionResult{}, nil)
	args := append([]string{"claim", "--id", "io-1", "--generation", "9", "--claim-token-file", "/tok", "--issue-body-sha256", "aa", "--context-packet-sha256", "bb", "--json"}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionClaim || got.Generation != 9 || got.TokenFile != "/tok" || got.ClaimCurrentToken ||
		got.IssueBodySHA256 != "aa" || got.ContextPacketSHA256 != "bb" || got.CWD != "/work" || got.Actor.Host != "codex" || got.Actor.SessionID != "sess-1" {
		t.Fatalf("claim req=%+v", got)
	}
	err := Run([]string{"claim", "--id", "io-1", "--json"}, deps)
	if err == nil || !strings.Contains(err.Error(), "exactly one of --claim-current-token or --claim-token-file") {
		t.Fatalf("no selector: %v", err)
	}
}

func TestReplaceRequiresExactlyOneActionAndExplicitOrNoActor(t *testing.T) {
	var got model.ExecutionActionRequest
	var printed []any
	deps := capturingDeps(&got, &printed, model.ExecutionReplaceResult{}, nil)
	for name, args := range map[string][]string{
		"none": {"replace", "--id", "io-1"},
		"two":  {"replace", "--id", "io-1", "--preview", "--revoke"},
	} {
		if err := Run(args, deps); err == nil || err.Error() != "execution replace requires exactly one action" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if err := Run([]string{"replace", "--id", "io-1", "--preview", "--host", "codex"}, deps); err == nil || !strings.Contains(err.Error(), "execution replace requires all ACTOR_FLAGS or none") {
		t.Fatalf("partial actor: %v", err)
	}
	if got.Action != "" {
		t.Fatalf("rejected replace executed: %+v", got)
	}
	args := append([]string{
		"replace", "--id", "io-1", "--finalize", "--expected-generation", "3", "--completion-generation", "2",
		"--inventory-fingerprint", "inv", "--quiescence-fingerprint", "q", "--reason", "why", "--confirm", "--json",
	}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	if got.Action != model.ExecutionActionReplace || got.ReplaceAction != model.ExecutionReplaceFinalize || got.ExpectedGeneration != 3 ||
		got.CompletionGeneration != 2 || got.InventoryFingerprint != "inv" || got.QuiescenceFingerprint != "q" || got.Reason != "why" || !got.Confirm || got.CWD != "/work" {
		t.Fatalf("replace req=%+v", got)
	}
}

func TestResumeRejectsPartialActorFlagsBeforeExecuting(t *testing.T) {
	var got model.ExecutionActionRequest
	var printed []any
	deps := capturingDeps(&got, &printed, nil, nil)
	err := Run([]string{"resume", "--id", "io-1", "--cwd", "/work"}, deps)
	if err == nil || !strings.Contains(err.Error(), "execution resume requires all ACTOR_FLAGS or none") {
		t.Fatalf("err=%v", err)
	}
	if got.Action != "" || len(printed) != 0 {
		t.Fatalf("resume executed despite rejection: %+v", got)
	}
}

func TestResolveNativeActorRejectsUnobservableAndInvalidObservations(t *testing.T) {
	good := func() nativeActorObservation {
		return nativeActorObservation{
			Getenv: func(k string) string {
				if k == "CODEX_THREAD_ID" {
					return "s"
				}
				return ""
			},
			Getwd: func() (string, error) { return "/work", nil },
			PID:   func() int { return 1 },
			ObserveAncestry: func(int) ([]model.NativeProcessReceipt, error) {
				return []model.NativeProcessReceipt{{PID: 1, Executable: "issueops"}, {PID: 2, Executable: "/opt/codex"}}, nil
			},
		}
	}
	flags, visited := resumeActorFlagsForTest(t, nil)
	if _, _, err := resolveNativeActor("claim", flags, visited, good()); err != nil {
		t.Fatalf("baseline observation rejected: %v", err)
	}
	obsErr := errors.New("ancestry boom")
	tests := map[string]struct {
		mutate func(*nativeActorObservation)
		want   string
	}{
		"missing getenv": {func(o *nativeActorObservation) { o.Getenv = nil }, "claim native actor observation is unavailable"},
		"missing getwd":  {func(o *nativeActorObservation) { o.Getwd = nil }, "native actor observation is unavailable"},
		"missing pid":    {func(o *nativeActorObservation) { o.PID = nil }, "native actor observation is unavailable"},
		"missing observe": {func(o *nativeActorObservation) { o.ObserveAncestry = nil },
			"native actor observation is unavailable"},
		"no identity": {func(o *nativeActorObservation) { o.Getenv = func(string) string { return "" } }, "native host session identity is unavailable"},
		"ancestry error": {func(o *nativeActorObservation) {
			o.ObserveAncestry = func(int) ([]model.NativeProcessReceipt, error) { return nil, obsErr }
		}, "ancestry boom"},
		"no host ancestor": {func(o *nativeActorObservation) {
			o.ObserveAncestry = func(int) ([]model.NativeProcessReceipt, error) {
				return []model.NativeProcessReceipt{{PID: 1, Executable: "issueops"}}, nil
			}
		}, "reusable codex host process ancestry is unavailable"},
		"getwd error":  {func(o *nativeActorObservation) { o.Getwd = func() (string, error) { return "", errors.New("gone") } }, "observe execution claim cwd: gone"},
		"relative cwd": {func(o *nativeActorObservation) { o.Getwd = func() (string, error) { return "rel/dir", nil } }, "execution claim cwd must be absolute"},
		"blank cwd":    {func(o *nativeActorObservation) { o.Getwd = func() (string, error) { return "  ", nil } }, "execution claim cwd must be absolute"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			obs := good()
			tt.mutate(&obs)
			_, _, err := resolveNativeActor("claim", flags, visited, obs)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err=%v want %q", err, tt.want)
			}
		})
	}
	// Surrounding whitespace and dot segments are normalized into a clean absolute cwd.
	obs := good()
	obs.Getwd = func() (string, error) { return " /work/./sub/.. ", nil }
	_, cwd, err := resolveNativeActor("claim", flags, visited, obs)
	if err != nil || cwd != "/work" {
		t.Fatalf("cwd=%q err=%v", cwd, err)
	}
}

func TestResolveNativeSessionIdentityReturnsHostSessionAndSource(t *testing.T) {
	env := map[string]string{"CLAUDE_CODE_SESSION_ID": "claude-1"}
	host, id, source, err := ResolveNativeSessionIdentity(func(k string) string { return env[k] })
	if err != nil || host != "claude" || id != "claude-1" || source != "CLAUDE_CODE_SESSION_ID" {
		t.Fatalf("got %q %q %q %v", host, id, source, err)
	}
	env["CODEX_THREAD_ID"] = "codex-1"
	host, id, source, err = ResolveNativeSessionIdentity(func(k string) string { return env[k] })
	if err == nil || host != "" || id != "" || source != "" {
		t.Fatalf("error must blank every identity value: %q %q %q %v", host, id, source, err)
	}
}

func clearNativeSessionEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"CODEX_THREAD_ID", "CLAUDE_CODE_SESSION_ID", "PI_SESSION_ID", "ISSUEOPS_OMP_SESSION_ID"} {
		t.Setenv(k, "")
	}
}

func TestWhoamiPrintsReusableReceiptAndFailsClosed(t *testing.T) {
	clearNativeSessionEnv(t)
	cwd := t.TempDir()
	t.Chdir(cwd)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	pid := os.Getpid()
	ancestry := []model.NativeProcessReceipt{
		{PID: pid, StartedAt: "t0", Executable: "issueops"},
		{PID: pid + 1000000, StartedAt: "t1", Executable: "/opt/codex"},
	}
	var printed []any
	var errPrinted []error
	deps := Deps{
		Runtime:    ExecutionDeps{ObserveNativeProcessAncestry: func(int) ([]model.NativeProcessReceipt, error) { return ancestry, nil }},
		PrintJSON:  func(v any) error { printed = append(printed, v); return nil },
		PrintError: func(e error) error { errPrinted = append(errPrinted, e); return nil },
	}

	if err := Run([]string{"whoami", "--json"}, deps); err == nil || !strings.Contains(err.Error(), "native host session identity is unavailable") {
		t.Fatalf("no identity: %v", err)
	}
	if len(errPrinted) != 1 || len(printed) != 0 {
		t.Fatalf("errPrinted=%d printed=%d", len(errPrinted), len(printed))
	}

	t.Setenv("CODEX_THREAD_ID", "codex-sess")
	if err := Run([]string{"whoami", "--json"}, deps); err != nil {
		t.Fatal(err)
	}
	result, ok := printed[0].(ExecutionWhoamiResult)
	if !ok || !result.OK || result.Host != "codex" || result.SessionID != "codex-sess" || result.SessionIDSource != "CODEX_THREAD_ID" {
		t.Fatalf("result=%#v", printed[0])
	}
	if len(result.Ancestry) != 1 || result.Ancestry[0].PID != pid+1000000 {
		t.Fatalf("ancestry=%+v", result.Ancestry)
	}
	if want := "--host codex --session-id 'codex-sess' --cwd '" + wd + "'"; result.RecordActorFlags != want {
		t.Fatalf("record flags=%q want %q", result.RecordActorFlags, want)
	}

	noHost := deps
	noHost.Runtime.ObserveNativeProcessAncestry = func(int) ([]model.NativeProcessReceipt, error) {
		return []model.NativeProcessReceipt{{PID: pid, Executable: "issueops"}}, nil
	}
	if err := Run([]string{"whoami", "--json"}, noHost); err == nil || !strings.Contains(err.Error(), "reusable codex host process ancestry is unavailable") {
		t.Fatalf("no host ancestor: %v", err)
	}
	boom := errors.New("observe boom")
	failing := deps
	failing.Runtime.ObserveNativeProcessAncestry = func(int) ([]model.NativeProcessReceipt, error) { return nil, boom }
	if err := Run([]string{"whoami", "--json"}, failing); !errors.Is(err, boom) {
		t.Fatalf("observe error: %v", err)
	}
	unconfigured := Deps{}
	if err := Run([]string{"whoami"}, unconfigured); err == nil || err.Error() != "issueops execution is not configured" {
		t.Fatalf("unconfigured: %v", err)
	}
}

func TestHandoffCmuxIsUnavailableWithoutHandlerOrStateRoot(t *testing.T) {
	args := []string{
		"handoff-cmux", "--id", "io-13", "--generation", "7",
		"--cmux-executable", "/bin/cmux", "--cmux-version", "1", "--cmux-build-identity", "b",
		"--cwd", "/w", "--socket", "/s.sock", "--window", "w", "--host", "omo", "--host-executable", "/bin/omo", "--model", "m",
		"--prompt-file", "/p", "--prompt-sha256", "aa", "--material-sha256", "bb",
	}
	handler := func(context.Context, string, model.ExecutionCmuxHandoffRequest) (model.ExecutionCmuxHandoffResult, error) {
		t.Fatal("handler must not run")
		return model.ExecutionCmuxHandoffResult{}, nil
	}
	if err := Run(args, Deps{StateRoot: func() string { return "/s" }}); err == nil || err.Error() != "execution handoff-cmux is unavailable" {
		t.Fatalf("no handler: %v", err)
	}
	if err := Run(args, Deps{HandoffCmux: handler}); err == nil || err.Error() != "execution handoff-cmux is unavailable" {
		t.Fatalf("no state root: %v", err)
	}
}

func TestSyncBaseRequiresExactlyOneModeAndConfiguredRuntime(t *testing.T) {
	deps := Deps{StateRoot: func() string { return "/s" }}
	for name, args := range map[string][]string{
		"none": {"sync-base", "--id", "io-1"},
		"two":  {"sync-base", "--id", "io-1", "--preview", "--abort"},
	} {
		if err := Run(args, deps); err == nil || err.Error() != "execution sync-base requires exactly one mode" {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Preview never requires the process cwd to match; it still needs a state root and runtime.
	if err := Run([]string{"sync-base", "--id", "io-1", "--preview"}, Deps{}); err == nil || err.Error() != "IssueOps state root is unavailable" {
		t.Fatalf("no state root: %v", err)
	}
	if err := Run([]string{"sync-base", "--id", "io-1", "--preview"}, deps); err == nil || err.Error() != "issueops execution is not configured" {
		t.Fatalf("no runtime: %v", err)
	}
}

func TestSyncBaseAbortAndFinalizeForwardModeWithoutConfirm(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	for flagName, wantMode := range map[string]string{"--abort": model.ExecutionSyncBaseAbort, "--finalize": model.ExecutionSyncBaseFinalize} {
		var got model.ExecutionSyncBaseRequest
		deps := Deps{
			StateRoot: func() string { return "/s" },
			Runtime: ExecutionDeps{SyncExecutionBase: func(_ context.Context, _ string, req model.ExecutionSyncBaseRequest, _ model.ExecutionSyncBaseDeps) (model.ExecutionSyncBaseResult, error) {
				got = req
				return model.ExecutionSyncBaseResult{OK: true}, nil
			}},
			PrintJSON: func(any) error { return nil },
		}
		args := append([]string{"sync-base", "--id", "io-1", flagName, "--fingerprint", "fp", "--json"}, explicitActorArgs[:len(explicitActorArgs)-2]...)
		args = append(args, "--cwd", cwd)
		if err := Run(args, deps); err != nil {
			t.Fatalf("%s: %v", flagName, err)
		}
		if got.Mode != wantMode || got.Confirm || got.Fingerprint != "fp" || got.CWD != cwd {
			t.Fatalf("%s: req=%+v", flagName, got)
		}
	}
}

func TestSameExistingExecutionPath(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	tests := []struct {
		name        string
		left, right string
		want        bool
	}{
		{"identical", dir, dir, true},
		{"symlink resolves to same dir", dir, link, true},
		{"trailing spaces trimmed", " " + dir + " ", dir, true},
		{"relative dot equals cwd", ".", dir, true},
		{"different dirs", dir, other, false},
		{"empty left", "", dir, false},
		{"blank right", dir, "   ", false},
		{"missing path", dir, filepath.Join(dir, "missing"), false},
		{"both missing", filepath.Join(dir, "a"), filepath.Join(dir, "a"), false},
	}
	for _, tt := range tests {
		if got := sameExistingExecutionPath(tt.left, tt.right); got != tt.want {
			t.Errorf("%s: got %t want %t", tt.name, got, tt.want)
		}
	}
}

func TestSwitchModeForwardsRequestAndFailsClosed(t *testing.T) {
	if err := Run([]string{"switch-mode", "--id", "io-1"}, Deps{}); err == nil || err.Error() != "IssueOps state root is unavailable" {
		t.Fatalf("no state root: %v", err)
	}
	if err := Run([]string{"switch-mode", "--id", "io-1"}, Deps{StateRoot: func() string { return "/s" }}); err == nil || err.Error() != "issueops execution is not configured" {
		t.Fatalf("no runtime: %v", err)
	}
	var got model.ExecutionSwitchModeRequest
	var printed []any
	deps := Deps{
		StateRoot: func() string { return "/state" },
		Runtime: ExecutionDeps{
			ObserveNativeProcessAncestry: fixedAncestry,
			SwitchExecutionMode: func(_ context.Context, root string, req model.ExecutionSwitchModeRequest) (model.ExecutionSwitchModeResult, error) {
				if root != "/state" {
					t.Fatalf("root=%q", root)
				}
				got = req
				return model.ExecutionSwitchModeResult{OK: true, ID: req.ID}, nil
			},
		},
		PrintJSON: func(v any) error { printed = append(printed, v); return nil },
	}
	args := append([]string{"switch-mode", "--id", "io-1", "--mode", "orca", "--fingerprint", "fp", "--apply", "--confirm", "--json"}, explicitActorArgs...)
	if err := Run(args, deps); err != nil {
		t.Fatal(err)
	}
	if got.ID != "io-1" || got.Mode != "orca" || got.Fingerprint != "fp" || !got.Apply || !got.Confirm || got.CWD != "/work" || got.Actor.SessionID != "sess-1" || len(printed) != 1 {
		t.Fatalf("req=%+v printed=%d", got, len(printed))
	}
}

type ownerInspectingOrca struct{ port.ExecutionOrcaProvisioner }

func (ownerInspectingOrca) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return port.ExecutionOrcaOwnerInventory{}, nil
}

type explicitOwnerInspector struct{}

func (explicitOwnerInspector) InspectOwner(context.Context, port.ExecutionOrcaOwnerInventoryRequest) (port.ExecutionOrcaOwnerInventory, error) {
	return port.ExecutionOrcaOwnerInventory{}, nil
}

func TestActionDepsDerivesOwnerInspectorOnlyFromCapableProvisioner(t *testing.T) {
	if got := (Deps{Orca: ownerInspectingOrca{}}).actionDeps().OrcaOwner; got == nil {
		t.Fatal("provisioner that can inspect owners was not promoted")
	}
	if got := (Deps{Orca: nil}).actionDeps().OrcaOwner; got != nil {
		t.Fatalf("owner inspector invented: %v", got)
	}
	explicit := explicitOwnerInspector{}
	got := (Deps{Orca: ownerInspectingOrca{}, OrcaOwner: explicit}).actionDeps().OrcaOwner
	if _, ok := got.(explicitOwnerInspector); !ok {
		t.Fatalf("explicit inspector replaced by %T", got)
	}
}

func TestOutputRoutesErrorsAndSuccessByJSONMode(t *testing.T) {
	boom := errors.New("boom")
	errPrinted := 0
	printed := 0
	deps := Deps{
		PrintJSON:  func(any) error { printed++; return nil },
		PrintError: func(error) error { errPrinted++; return nil },
	}
	if err := output(nil, true, boom, deps); !errors.Is(err, boom) || errPrinted != 1 {
		t.Fatalf("json error: err=%v errPrinted=%d", err, errPrinted)
	}
	if err := output(nil, false, boom, deps); !errors.Is(err, boom) || errPrinted != 1 {
		t.Fatalf("text error must not use the JSON error printer: err=%v errPrinted=%d", err, errPrinted)
	}
	if err := output(nil, true, boom, Deps{}); !errors.Is(err, boom) {
		t.Fatalf("json error without printer: %v", err)
	}
	if err := output("v", true, nil, deps); err != nil || printed != 1 {
		t.Fatalf("json success: err=%v printed=%d", err, printed)
	}
	out := testsupport.CaptureStdout(t, func() error { return output("plain", false, nil, deps) })
	if out != "plain\n" || printed != 1 {
		t.Fatalf("text success out=%q printed=%d", out, printed)
	}
	out = testsupport.CaptureStdout(t, func() error { return output("noprinter", true, nil, Deps{}) })
	if out != "noprinter\n" {
		t.Fatalf("json without printer falls back to text: %q", out)
	}
	printErr := errors.New("print boom")
	if err := output("v", true, nil, Deps{PrintJSON: func(any) error { return printErr }}); !errors.Is(err, printErr) {
		t.Fatalf("print error not propagated: %v", err)
	}
}

func TestOutputFailsClosedWhenCommandProvenanceCannotBeBound(t *testing.T) {
	errPrinted := 0
	deps := Deps{PrintJSON: func(any) error { t.Fatal("printed unbound command"); return nil }, PrintError: func(error) error { errPrinted++; return nil }}
	value := model.ExecutionResult{Execution: model.Execution{Lease: model.WriteLease{Generation: 2}}, NextCommand: "issueops execution release --id io-1"}
	if err := output(value, true, nil, deps); err == nil || errPrinted != 1 {
		t.Fatalf("err=%v errPrinted=%d", err, errPrinted)
	}
}

func TestPrintTextFormatsEachResultKind(t *testing.T) {
	gen := func(n uint64, status model.LeaseStatus) model.Execution {
		return model.Execution{Lease: model.WriteLease{Generation: n, Status: status}}
	}
	exec := gen(4, "claimed")
	prepare := model.ExecutionPrepareResult{ID: "io-1", ResolvedMode: "orca"}
	prepare.Workspace.Root = "/ws"
	var cmux model.ExecutionCmuxHandoffResult
	cmux.ID, cmux.Status = "io-1", "delivered"
	cmux.Target.WindowID, cmux.Target.WorkspaceID, cmux.Target.SurfaceID = "w", "ws", "sf"
	pending := &model.ExternalIntent{}
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"prepare without execution", prepare, "io-1 orca /ws generation=0\n"},
		{"prepare with execution", func() any { p := prepare; p.Execution = &exec; return p }(), "io-1 orca /ws generation=4\n"},
		{"execution", model.ExecutionResult{ID: "io-1", Execution: exec}, "io-1 claimed generation=4\n"},
		{"replace", model.ExecutionReplaceResult{ID: "io-1", Execution: exec, NextCommand: "next"}, "io-1 claimed generation=4 next=next\n"},
		{"reconcile pending", model.ExecutionReconcileResult{ID: "io-1", Code: "c", Pending: pending}, "io-1 c pending=true\n"},
		{"reconcile settled", model.ExecutionReconcileResult{ID: "io-1", Code: "c"}, "io-1 c pending=false\n"},
		{"sync-base", model.ExecutionSyncBaseResult{ID: "io-1", Mode: "apply", Merged: true, Pushed: false, ConflictFiles: []string{"a", "b"}, NextCommand: "n"}, "io-1 apply merged=true pushed=false conflicts=2 next=n\n"},
		{"cmux", cmux, "io-1 cmux delivered window=w workspace=ws surface=sf\n"},
		{"fallback", 42, "42\n"},
	}
	for _, tt := range tests {
		out := testsupport.CaptureStdout(t, func() error { printText(tt.value); return nil })
		if out != tt.want {
			t.Errorf("%s: got %q want %q", tt.name, out, tt.want)
		}
	}
}

func TestExecutionGenerationIsZeroWithoutExecution(t *testing.T) {
	if executionGeneration(nil) != 0 {
		t.Fatal("nil execution must have generation 0")
	}
	if got := executionGeneration(&model.Execution{Lease: model.WriteLease{Generation: 9}}); got != 9 {
		t.Fatalf("generation=%d", got)
	}
}

func TestRepeatedStringAccumulatesInOrder(t *testing.T) {
	var values repeatedString
	_ = values.Set("a")
	_ = values.Set("b")
	if values.String() != "a,b" || !reflect.DeepEqual([]string(values), []string{"a", "b"}) {
		t.Fatalf("values=%v", values)
	}
}

func TestExecutionNextCommandBindsResultAndResumeAndSkipsUnleasedAndUnknown(t *testing.T) {
	observer := provenanceObserverStub{evidence: provenanceport.Receipt{ExecutablePath: "/repo/bin/issueops", ExecutableSHA256: strings.Repeat("c", 64)}}
	leased := model.Execution{Lease: model.WriteLease{Generation: 8}}
	for name, value := range map[string]any{
		"execution": model.ExecutionResult{Execution: leased, NextCommand: "issueops execution release --id io-1 --generation 8"},
		"resume":    model.ExecutionResumeResult{Execution: leased, NextCommand: "issueops execution release --id io-1 --generation 8"},
	} {
		bound, err := bindExecutionNextCommand(value, observer)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var command string
		switch r := bound.(type) {
		case model.ExecutionResult:
			command = r.NextCommand
		case model.ExecutionResumeResult:
			command = r.NextCommand
		default:
			t.Fatalf("%s: type %T", name, bound)
		}
		if !strings.HasPrefix(command, "'/repo/bin/issueops'") || !strings.Contains(command, "--generated-for-generation 8") {
			t.Fatalf("%s: command=%q", name, command)
		}
	}
	// Without a lease generation or a command nothing is bound, even with no observer.
	for name, value := range map[string]any{
		"unleased execution": model.ExecutionResult{NextCommand: "issueops execution claim"},
		"empty command":      model.ExecutionResult{Execution: leased},
		"unknown type":       "plain",
	} {
		bound, err := bindExecutionNextCommand(value, nil)
		if err != nil || !reflect.DeepEqual(bound, value) {
			t.Fatalf("%s: bound=%#v err=%v", name, bound, err)
		}
	}
}

func TestReadExecutionIssueSnapshotFileEmptyPathAndMalformedContent(t *testing.T) {
	got, err := readExecutionIssueSnapshotFile("  ")
	if got != nil || err != nil {
		t.Fatalf("blank path: %v %v", got, err)
	}
	if _, err := readExecutionIssueSnapshotFile(filepath.Join(t.TempDir(), "none.json")); err == nil || !strings.Contains(err.Error(), "inspect issue snapshot file") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := readExecutionIssueSnapshotFile(writeExecutionSnapshotTestFile(t, "null", 0o600)); err == nil || err.Error() != "issue snapshot file must contain one JSON object" {
		t.Fatalf("null: %v", err)
	}
	if _, err := readExecutionIssueSnapshotFile(writeExecutionSnapshotTestFile(t, validExecutionSnapshotJSON()+" {garbage", 0o600)); err == nil || !strings.Contains(err.Error(), "decode trailing issue snapshot data") {
		t.Fatalf("garbage trailer: %v", err)
	}
	if _, err := readExecutionIssueSnapshotFile(writeExecutionSnapshotTestFile(t, validExecutionSnapshotJSON()+" 7", 0o600)); err == nil || err.Error() != "issue snapshot file must contain exactly one JSON object" {
		t.Fatalf("second value: %v", err)
	}
	ok, err := readExecutionIssueSnapshotFile(writeExecutionSnapshotTestFile(t, validExecutionSnapshotJSON()+"\n", 0o600))
	if err != nil || ok == nil || ok.Source != "glab_mcp" {
		t.Fatalf("valid: %+v %v", ok, err)
	}
}
