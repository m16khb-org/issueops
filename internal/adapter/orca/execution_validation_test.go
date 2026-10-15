package orca

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/port"
)

func TestValidateExecutionPrepareRejectsEachMissingSeal(t *testing.T) {
	workspace, probe := executionFixture(t)
	tests := []struct {
		name    string
		mutate  func(*port.ExecutionWorkspaceRequest, *port.ExecutionOrcaProbeRequest)
		wantErr string
	}{
		{"valid", func(*port.ExecutionWorkspaceRequest, *port.ExecutionOrcaProbeRequest) {}, ""},
		{"valid without parent", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) { w.ParentWorktree = "" }, ""},
		{"valid gitlab", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) {
			*p = executionGitLabProbe(*p)
		}, ""},
		{"no lifecycle", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) { w.LifecycleID = " " }, "exact lifecycle and workspace identity"},
		{"relative source root", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) { w.SourceRoot = "repo" }, "exact lifecycle and workspace identity"},
		{"relative worktree root", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) { w.Root = "wt" }, "exact lifecycle and workspace identity"},
		{"no branch", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) { w.Branch = "" }, "exact lifecycle and workspace identity"},
		{"no base head", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) { w.BaseHead = " " }, "exact lifecycle and workspace identity"},
		{"unknown host", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) { p.Host = "gemini" }, "explicit model and marker"},
		{"no model", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) { p.Model = " " }, "explicit model and marker"},
		{"no marker", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) { p.Marker = "" }, "explicit model and marker"},
		{"unknown provider", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) { p.Provider = "bitbucket" }, "github or gitlab issue identity"},
		{"no issue", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) { p.Issue = 0 }, "positive issue number"},
		{"negative issue", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) { p.Issue = -1 }, "positive issue number"},
		{"marker without issue field", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) {
			p.Marker = "issueops-v1 lifecycle=io-69 provider=github"
		}, "does not seal the exact provider and issue number"},
		{"marker with empty issue value", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) {
			p.Marker = "issueops-v1 provider=github issue="
		}, "does not seal the exact provider and issue number"},
		{"marker repeating the issue field", func(_ *port.ExecutionWorkspaceRequest, p *port.ExecutionOrcaProbeRequest) {
			p.Marker += " issue=69"
		}, "does not seal the exact provider and issue number"},
		{"relative parent", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) {
			w.ParentWorktree = "../parent"
		}, "isolated absolute path"},
		{"parent is the source root", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) {
			w.ParentWorktree = w.SourceRoot
		}, "isolated absolute path"},
		{"parent is the worktree itself", func(w *port.ExecutionWorkspaceRequest, _ *port.ExecutionOrcaProbeRequest) {
			w.ParentWorktree = w.Root + "/"
		}, "isolated absolute path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, p := workspace, probe
			tt.mutate(&w, &p)
			err := validateExecutionPrepare(w, p)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected rejection: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestExecutionMarkerFieldRequiresOneNonEmptyValue(t *testing.T) {
	for _, tt := range []struct {
		marker, name, want string
		ok                 bool
	}{
		{"a=1 b=2", "b", "2", true},
		{"a=1  b=2", "a", "1", true},
		{"a=1 b=2", "c", "", false},
		{"a=1 a=2", "a", "", false},
		{"a= b=2", "a", "", false},
		{"aa=1", "a", "", false},
	} {
		got, ok := executionMarkerField(tt.marker, tt.name)
		if got != tt.want || ok != tt.ok {
			t.Errorf("executionMarkerField(%q, %q) = %q, %t; want %q, %t", tt.marker, tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestExecutionPathInsideRootRejectsEscapesAndRelativePaths(t *testing.T) {
	for _, tt := range []struct {
		root, path string
		want       bool
	}{
		{"/r", "/r/a/b", true},
		{"/r/", "/r/./a", true},
		{" /r ", " /r/a ", true},
		{"/r", "/r", false},
		{"/r", "/r/..", false},
		{"/r", "/r/../x", false},
		{"/r", "/rx/a", false},
		{"/r", "/other", false},
		{"r", "/r/a", false},
		{"/r", "r/a", false},
	} {
		if got := executionPathInsideRoot(tt.root, tt.path); got != tt.want {
			t.Errorf("executionPathInsideRoot(%q, %q) = %t, want %t", tt.root, tt.path, got, tt.want)
		}
	}
}

func TestValidExecutionSHA256(t *testing.T) {
	good := strings.Repeat("ab", 32)
	for _, tt := range []struct {
		value string
		want  bool
	}{
		{good, true},
		{strings.ToUpper(good), true},
		{" " + good + "\n", true},
		{good[:63], false},
		{good + "0", false},
		{strings.Repeat("g", 64), false},
		{"", false},
	} {
		if got := validExecutionSHA256(tt.value); got != tt.want {
			t.Errorf("validExecutionSHA256(%q) = %t, want %t", tt.value, got, tt.want)
		}
	}
}

func TestInspectionRequiresASealedOwnerEnvelope(t *testing.T) {
	fx := newStageFixture(t)
	good := fx.request(port.ExecutionOrcaIntentTerminal)
	tests := []struct {
		name    string
		mutate  func(*port.ExecutionOrcaIntentRequest)
		wantErr string
	}{
		{"complete", func(*port.ExecutionOrcaIntentRequest) {}, ""},
		{"no worktree receipt", func(r *port.ExecutionOrcaIntentRequest) { r.Prepared = nil }, "sealed worktree and launch receipts"},
		{"no launch receipt", func(r *port.ExecutionOrcaIntentRequest) { r.Launch = nil }, "sealed worktree and launch receipts"},
		{"worktree id blank", func(r *port.ExecutionOrcaIntentRequest) { r.Prepared.WorktreeID = " " }, "worktree receipt is incomplete"},
		{"runtime id blank", func(r *port.ExecutionOrcaIntentRequest) { r.Prepared.RuntimeID = "" }, "worktree receipt is incomplete"},
		{"repo id blank", func(r *port.ExecutionOrcaIntentRequest) { r.Prepared.RepoID = "" }, "worktree receipt is incomplete"},
		{"prompt digest malformed", func(r *port.ExecutionOrcaIntentRequest) { r.Launch.PromptSHA256 = "abc" }, "launch receipt is incomplete"},
		{"packet digest malformed", func(r *port.ExecutionOrcaIntentRequest) { r.Launch.ContextPacketSHA256 = strings.Repeat("z", 64) }, "launch receipt is incomplete"},
		{"prompt outside the worktree", func(r *port.ExecutionOrcaIntentRequest) { r.Launch.PromptPath = "/elsewhere/prompt.txt" }, "launch receipt is incomplete"},
		{"packet outside the worktree", func(r *port.ExecutionOrcaIntentRequest) { r.Launch.ContextPacketPath = r.Workspace.Root }, "launch receipt is incomplete"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := good
			prepared, launch := *good.Prepared, *good.Launch
			request.Prepared, request.Launch = &prepared, &launch
			tt.mutate(&request)
			_, err := NewExecutionClient(fx.fake()).InspectIntent(context.Background(), request)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected rejection: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestIntentRequestsRejectMalformedRetryRequestIDs(t *testing.T) {
	fx := newStageFixture(t)
	for name, mutate := range map[string]func(*port.ExecutionOrcaIntentRequest){
		"dispatch retry id": func(r *port.ExecutionOrcaIntentRequest) { r.RetryRequestID = "not-a-uuid" },
		"prompt retry id":   func(r *port.ExecutionOrcaIntentRequest) { r.PromptRetryRequestID = "not-a-uuid" },
	} {
		t.Run(name, func(t *testing.T) {
			request := fx.request(port.ExecutionOrcaIntentDispatch)
			mutate(&request)
			client := fx.fake()
			_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
			orcaErr := requireOrcaError(t, err, "intent_preflight_rejected")
			if orcaErr.Invoked || !strings.Contains(orcaErr.Detail, "request UUID") || len(client.calls) != 0 {
				t.Fatalf("error=%#v calls=%v", orcaErr, client.calls)
			}
			if _, err := NewExecutionClient(client).InspectIntent(context.Background(), request); err == nil {
				t.Fatal("inspection accepted a malformed retry request id")
			}
		})
	}
}

func TestInvokeOwnerStageWithoutLaunchIsPreflightRejected(t *testing.T) {
	fx := newStageFixture(t)
	request := fx.request(port.ExecutionOrcaIntentTerminal)
	request.Launch = nil
	client := fx.fake()
	_, err := NewExecutionClient(client).InvokeIntent(context.Background(), request)
	if orcaErr := requireOrcaError(t, err, "intent_preflight_rejected"); !strings.Contains(orcaErr.Detail, "sealed context packet is invalid") {
		t.Fatalf("detail = %q", orcaErr.Detail)
	}
	if len(client.calls) != 0 {
		t.Fatalf("a launch-less owner stage reached Orca: %v", client.calls)
	}
}

// sealedLaunch writes a packet and prompt under root and returns the matching
// launch receipt so each test can break exactly one property.
func sealedLaunch(t *testing.T, root string, promptFor func(packetPath, packetDigest string) string) port.ExecutionOrcaLaunchRequest {
	t.Helper()
	dir := filepath.Join(root, ".issueops", "state")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	packet := []byte(`{"schema_version":1}`)
	packetPath := filepath.Join(dir, "context.json")
	if err := os.WriteFile(packetPath, packet, 0o600); err != nil {
		t.Fatal(err)
	}
	packetDigest := digestExecutionBytes(packet)
	prompt := promptFor(packetPath, packetDigest)
	promptPath := filepath.Join(dir, "prompt.txt")
	if err := os.WriteFile(promptPath, []byte(prompt), 0o600); err != nil {
		t.Fatal(err)
	}
	return port.ExecutionOrcaLaunchRequest{
		Prompt: prompt, PromptPath: promptPath, PromptSHA256: digestExecutionBytes([]byte(prompt)),
		ContextPacketPath: packetPath, ContextPacketSHA256: packetDigest,
	}
}

func boundPrompt(packetPath, packetDigest string) string {
	return "owner prompt\npacket=" + packetPath + "\ndigest=" + packetDigest
}

func TestValidateExecutionOwnerLaunch(t *testing.T) {
	root := t.TempDir()
	prepared := port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{Root: root}, RuntimeID: "runtime-1", RepoID: "repo-1", WorktreeID: "wt-1",
	}
	probe := port.ExecutionOrcaProbeRequest{Host: "codex", Model: "gpt"}
	good := sealedLaunch(t, root, boundPrompt)

	t.Run("sealed launch passes", func(t *testing.T) {
		if err := validateExecutionOwnerLaunch(prepared, probe, good); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("digests are compared case-insensitively", func(t *testing.T) {
		upperRoot := t.TempDir()
		upper := sealedLaunch(t, upperRoot, func(path, digest string) string { return boundPrompt(path, strings.ToUpper(digest)) })
		upper.PromptSHA256 = " " + strings.ToUpper(upper.PromptSHA256) + " "
		upper.ContextPacketSHA256 = strings.ToUpper(upper.ContextPacketSHA256)
		upperPrepared := prepared
		upperPrepared.Workspace.Root = upperRoot
		if err := validateExecutionOwnerLaunch(upperPrepared, probe, upper); err != nil {
			t.Fatalf("upper-case digests were rejected: %v", err)
		}
	})
	for _, tt := range []struct {
		name    string
		mutate  func(*port.ExecutionOrcaWorkspaceReceipt, *port.ExecutionOrcaProbeRequest, *port.ExecutionOrcaLaunchRequest)
		wantErr string
	}{
		{"receipt without worktree id", func(p *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, _ *port.ExecutionOrcaLaunchRequest) {
			p.WorktreeID = ""
		}, "workspace receipt is incomplete"},
		{"receipt without runtime", func(p *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, _ *port.ExecutionOrcaLaunchRequest) {
			p.RuntimeID = " "
		}, "workspace receipt is incomplete"},
		{"receipt without repo", func(p *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, _ *port.ExecutionOrcaLaunchRequest) {
			p.RepoID = ""
		}, "workspace receipt is incomplete"},
		{"unknown host", func(_ *port.ExecutionOrcaWorkspaceReceipt, r *port.ExecutionOrcaProbeRequest, _ *port.ExecutionOrcaLaunchRequest) {
			r.Host = "gemini"
		}, "first-party owner profile"},
		{"blank model", func(_ *port.ExecutionOrcaWorkspaceReceipt, r *port.ExecutionOrcaProbeRequest, _ *port.ExecutionOrcaLaunchRequest) {
			r.Model = " "
		}, "first-party owner profile"},
		{"packet missing", func(_ *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, l *port.ExecutionOrcaLaunchRequest) {
			l.ContextPacketPath = filepath.Join(root, "absent.json")
		}, "sealed context packet is invalid"},
		{"packet digest differs", func(_ *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, l *port.ExecutionOrcaLaunchRequest) {
			l.ContextPacketSHA256 = strings.Repeat("0", 64)
		}, "sealed context packet digest mismatch"},
		{"prompt file missing", func(_ *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, l *port.ExecutionOrcaLaunchRequest) {
			l.PromptPath = filepath.Join(root, "absent.txt")
		}, "sealed owner prompt is invalid"},
		{"prompt text differs from the sealed file", func(_ *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, l *port.ExecutionOrcaLaunchRequest) {
			l.Prompt += " edited"
		}, "sealed owner prompt digest mismatch"},
		{"prompt digest differs", func(_ *port.ExecutionOrcaWorkspaceReceipt, _ *port.ExecutionOrcaProbeRequest, l *port.ExecutionOrcaLaunchRequest) {
			l.PromptSHA256 = strings.Repeat("0", 64)
		}, "sealed owner prompt digest mismatch"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, r, l := prepared, probe, good
			tt.mutate(&p, &r, &l)
			err := validateExecutionOwnerLaunch(p, r, l)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}

	for name, promptFor := range map[string]func(string, string) string{
		"unresolved placeholder": func(path, digest string) string { return boundPrompt(path, digest) + "\n{ISSUE_NUMBER}" },
		"packet path not bound":  func(_, digest string) string { return "owner prompt\ndigest=" + digest },
		"packet digest not bound": func(path, _ string) string {
			return "owner prompt\npacket=" + path
		},
	} {
		t.Run(name, func(t *testing.T) {
			launch := sealedLaunch(t, root, promptFor)
			err := validateExecutionOwnerLaunch(prepared, probe, launch)
			if err == nil || !strings.Contains(err.Error(), "unresolved or does not bind the sealed packet") {
				t.Fatalf("error = %v", err)
			}
		})
	}
	t.Run("lowercase braces are not placeholders", func(t *testing.T) {
		launch := sealedLaunch(t, root, func(path, digest string) string { return boundPrompt(path, digest) + "\n{json}" })
		if err := validateExecutionOwnerLaunch(prepared, probe, launch); err != nil {
			t.Fatal(err)
		}
	})
}

func TestReadExecutionSealedFileGuards(t *testing.T) {
	root := t.TempDir()
	write := func(rel string, mode os.FileMode, content []byte) string {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		return path
	}
	private := write("ok/private.txt", 0o600, []byte("sealed"))
	write("ok/group-readable.txt", 0o640, []byte("x"))
	write("ok/world-readable.txt", 0o604, []byte("x"))
	write("file-as-dir", 0o600, []byte("x"))
	write("big/exact.bin", 0o600, make([]byte, 1<<20))
	write("big/over.bin", 0o600, make([]byte, 1<<20+1))
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "ok", "link-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Dir(outside), filepath.Join(root, "link-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ok", "subdir"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := readExecutionSealedFile(root, private)
	if err != nil || string(got) != "sealed" {
		t.Fatalf("private file = %q, %v", got, err)
	}
	if _, err := readExecutionSealedFile(root, filepath.Join(root, "big", "exact.bin")); err != nil {
		t.Fatalf("a file exactly at the size limit must be accepted: %v", err)
	}
	for _, tt := range []struct {
		name, path, want string
	}{
		{"the root itself", root, "inside the canonical worktree"},
		{"parent escape", filepath.Join(root, "..", "x"), "inside the canonical worktree"},
		{"another tree", outside, "inside the canonical worktree"},
		{"missing file", filepath.Join(root, "ok", "absent.txt"), "real directories and a real file"},
		{"symlinked file", filepath.Join(root, "ok", "link-file"), "real directories and a real file"},
		{"symlinked directory", filepath.Join(root, "link-dir", "outside.txt"), "real directories and a real file"},
		{"ancestor is a file", filepath.Join(root, "file-as-dir", "child"), "ancestor must be a directory"},
		{"directory instead of a file", filepath.Join(root, "ok", "subdir"), "regular, private"},
		{"group readable", filepath.Join(root, "ok", "group-readable.txt"), "regular, private"},
		{"world readable", filepath.Join(root, "ok", "world-readable.txt"), "regular, private"},
		{"over the size limit", filepath.Join(root, "big", "over.bin"), "regular, private"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readExecutionSealedFile(root, tt.path)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateExecutionResolvedTerminalRelaxesOnlyAnEmptyStableTitle(t *testing.T) {
	prepared := diagnosisPrepared()
	terminal := diagnosisTerminal()
	const marker = "issueops-v1 marker"

	terminal.Title, terminal.StableTabTitle = "✳ Claude Code", ""
	if err := validateExecutionResolvedTerminal(terminal, prepared, marker); err != nil {
		t.Fatalf("an agent-overwritten title with no stable title must be accepted: %v", err)
	}
	terminal.StableTabTitle = "another lifecycle's marker"
	err := validateExecutionResolvedTerminal(terminal, prepared, marker)
	if err == nil || !strings.Contains(err.Error(), "tab title mismatch") || !strings.Contains(err.Error(), "another lifecycle's marker") {
		t.Fatalf("a different stable title must be rejected with both observations: %v", err)
	}
	terminal.StableTabTitle = marker
	if err := validateExecutionResolvedTerminal(terminal, prepared, marker); err != nil {
		t.Fatalf("a matching stable title must be accepted: %v", err)
	}
	terminal.Connected = false
	if err := validateExecutionResolvedTerminal(terminal, prepared, marker); err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("receipt axes must still be enforced: %v", err)
	}
	if err := validateExecutionTerminalReceipt(diagnosisTerminal(), port.ExecutionOrcaWorkspaceReceipt{}); err == nil || !strings.Contains(err.Error(), "sealed runtime id is empty") {
		t.Fatalf("unsealed runtime error = %v", err)
	}
}

func TestExecutionResourceValidators(t *testing.T) {
	prepared := port.ExecutionOrcaWorkspaceReceipt{RuntimeID: "rt"}
	t.Run("run", func(t *testing.T) {
		good := port.OrcaRun{RuntimeID: "rt", ID: "run", Objective: " objective "}
		if err := validateExecutionIntentRun(good, prepared, "objective"); err != nil {
			t.Fatalf("matching Run rejected: %v", err)
		}
		for name, mutate := range map[string]func(*port.OrcaRun){
			"blank id":       func(r *port.OrcaRun) { r.ID = " " },
			"other runtime":  func(r *port.OrcaRun) { r.RuntimeID = "other" },
			"other objctive": func(r *port.OrcaRun) { r.Objective = "different" },
		} {
			run := good
			mutate(&run)
			if err := validateExecutionIntentRun(run, prepared, "objective"); err == nil {
				t.Errorf("%s: mismatching Run accepted", name)
			}
		}
		if err := validateExecutionIntentRun(good, port.ExecutionOrcaWorkspaceReceipt{}, "objective"); err == nil {
			t.Error("a Run was accepted against an unsealed runtime")
		}
	})
	t.Run("task", func(t *testing.T) {
		good := port.OrcaTask{RuntimeID: "rt", RunID: "run", ID: "task", Title: "title", DisplayName: "branch"}
		if err := validateExecutionIntentTask(good, prepared, " run ", " title ", " branch "); err != nil {
			t.Fatalf("matching task rejected: %v", err)
		}
		for name, mutate := range map[string]func(*port.OrcaTask){
			"blank id":      func(x *port.OrcaTask) { x.ID = "" },
			"other runtime": func(x *port.OrcaTask) { x.RuntimeID = "other" },
			"other run":     func(x *port.OrcaTask) { x.RunID = "other" },
			"other title":   func(x *port.OrcaTask) { x.Title = "other" },
			"other display": func(x *port.OrcaTask) { x.DisplayName = "other" },
		} {
			task := good
			mutate(&task)
			if err := validateExecutionIntentTask(task, prepared, "run", "title", "branch"); err == nil {
				t.Errorf("%s: mismatching task accepted", name)
			}
		}
	})
	t.Run("invoked dispatch", func(t *testing.T) {
		good := port.OrcaDispatch{RuntimeID: "rt", ID: "d", TaskID: "task", AssigneeHandle: "term", Injected: true}
		if err := validateExecutionInvokedDispatch(good, "rt", "task", "term", true); err != nil {
			t.Fatalf("matching dispatch rejected: %v", err)
		}
		withPreamble := good
		withPreamble.Injected, withPreamble.Preamble = false, "preamble"
		if err := validateExecutionInvokedDispatch(withPreamble, "rt", "task", "term", false); err != nil {
			t.Fatalf("matching non-injected dispatch rejected: %v", err)
		}
		for name, mutate := range map[string]func(*port.OrcaDispatch){
			"blank id":        func(d *port.OrcaDispatch) { d.ID = "" },
			"other runtime":   func(d *port.OrcaDispatch) { d.RuntimeID = "other" },
			"other task":      func(d *port.OrcaDispatch) { d.TaskID = "other" },
			"other assignee":  func(d *port.OrcaDispatch) { d.AssigneeHandle = "other" },
			"inject mismatch": func(d *port.OrcaDispatch) { d.Injected = false },
		} {
			dispatch := good
			mutate(&dispatch)
			if err := validateExecutionInvokedDispatch(dispatch, "rt", "task", "term", true); err == nil {
				t.Errorf("%s: mismatching dispatch accepted", name)
			}
		}
		if err := validateExecutionInvokedDispatch(good, "rt", "task", " ", true); err == nil {
			t.Error("a dispatch was accepted for a blank terminal handle")
		}
		if err := validateExecutionInvokedDispatch(good, " ", "task", "term", true); err == nil {
			t.Error("a dispatch was accepted for a blank sealed runtime")
		}
	})
	t.Run("observed dispatch", func(t *testing.T) {
		good := port.OrcaDispatch{RuntimeID: "rt", ID: "d", TaskID: "task", AssigneeHandle: "term", Status: "dispatched", RequestID: "req"}
		if err := validateExecutionObservedDispatch(good, "rt", "task", "term", "req"); err != nil {
			t.Fatalf("matching dispatch rejected: %v", err)
		}
		for name, mutate := range map[string]func(*port.OrcaDispatch){
			"blank id":       func(d *port.OrcaDispatch) { d.ID = "" },
			"other runtime":  func(d *port.OrcaDispatch) { d.RuntimeID = "other" },
			"other task":     func(d *port.OrcaDispatch) { d.TaskID = "other" },
			"other assignee": func(d *port.OrcaDispatch) { d.AssigneeHandle = "other" },
			"other request":  func(d *port.OrcaDispatch) { d.RequestID = "other" },
			"blank status":   func(d *port.OrcaDispatch) { d.Status = " " },
		} {
			dispatch := good
			mutate(&dispatch)
			if err := validateExecutionObservedDispatch(dispatch, "rt", "task", "term", "req"); err == nil {
				t.Errorf("%s: mismatching dispatch accepted", name)
			}
		}
		if err := validateExecutionObservedDispatch(good, "rt", "task", "term", " "); err == nil {
			t.Error("a dispatch was accepted without a request identity")
		}
	})
	t.Run("inventory runtime", func(t *testing.T) {
		for _, tt := range []struct {
			observed, sealed string
			ok               bool
		}{{"rt", "rt", true}, {"rt", "other", false}, {"", "", false}, {" ", "rt", false}, {"rt", " ", false}} {
			if err := validateExecutionInventoryRuntime(tt.observed, tt.sealed); (err == nil) != tt.ok {
				t.Errorf("validateExecutionInventoryRuntime(%q, %q) = %v, want ok=%t", tt.observed, tt.sealed, err, tt.ok)
			}
		}
	})
}

func TestExplicitExecutionParentLineage(t *testing.T) {
	good := port.OrcaWorktree{RepoID: "repo", ParentWorktreeID: "repo::/parent", LineageConfidence: "explicit", LineageSource: "explicit-cli-flag"}
	for _, tt := range []struct {
		name   string
		mutate func(*port.OrcaWorktree)
		want   bool
	}{
		{"create flag", func(*port.OrcaWorktree) {}, true},
		{"manual update", func(w *port.OrcaWorktree) { w.LineageSource = "manual-action" }, true},
		{"inferred confidence", func(w *port.OrcaWorktree) { w.LineageConfidence = "inferred" }, false},
		{"unknown source", func(w *port.OrcaWorktree) { w.LineageSource = "heuristic" }, false},
		{"parent id without repo separator", func(w *port.OrcaWorktree) { w.ParentWorktreeID = "/parent" }, false},
		{"parent from another repo", func(w *port.OrcaWorktree) { w.ParentWorktreeID = "other::/parent" }, false},
		{"another parent path", func(w *port.OrcaWorktree) { w.ParentWorktreeID = "repo::/different" }, false},
	} {
		row := good
		tt.mutate(&row)
		if got := explicitExecutionParentLineage(row, "/parent"); got != tt.want {
			t.Errorf("%s: lineage proof = %t, want %t", tt.name, got, tt.want)
		}
	}
}
