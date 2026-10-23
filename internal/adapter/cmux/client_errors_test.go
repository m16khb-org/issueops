package cmux

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cmuxcontract "issueops/internal/contract/cmux"
	issueops "issueops/internal/contract/issueops"
	cmuxport "issueops/internal/port/cmux"
)

func preflightRequestFixture() cmuxcontract.PreflightRequest {
	return cmuxcontract.PreflightRequest{Executable: cmuxPath, ExpectedVersion: SupportedVersion, ExpectedBuild: SupportedBuildIdentity, SocketPath: socketPath, WindowID: testWindow}
}

func okPreflightSteps() []runnerStep {
	return []runnerStep{
		{args: []string{"version"}, stdout: SupportedBuildIdentity},
		{args: []string{"ping"}, stdout: "PONG\n"},
		{args: []string{"capabilities"}, stdout: capabilitiesFixture()},
		{args: []string{"--id-format", "uuids", "identify", "--window", testWindow, "--no-caller"}, stdout: identifyFixture(testWindow, "", "")},
	}
}

func TestPreflightRejectsIncompleteRequestsBeforeAnyCommandOrObservation(t *testing.T) {
	observed := 0
	observer := func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) {
		observed++
		return endpointFixture(), nil
	}
	tests := []struct {
		name   string
		client Client
		mutate func(*cmuxcontract.PreflightRequest)
		want   string
	}{
		{name: "no runner", client: Client{ObserveEndpoint: observer}, want: "dependencies"},
		{name: "no observer", client: Client{Runner: &queueRunner{t: t}}, want: "dependencies"},
		{name: "relative executable", mutate: func(r *cmuxcontract.PreflightRequest) { r.Executable = "cmux" }, want: "absolute clean"},
		{name: "unclean executable", mutate: func(r *cmuxcontract.PreflightRequest) { r.Executable = "/Applications/../bin/cmux" }, want: "absolute clean"},
		{name: "unsupported version", mutate: func(r *cmuxcontract.PreflightRequest) { r.ExpectedVersion = "0.64.9" }, want: "identity is incomplete"},
		{name: "unsupported build", mutate: func(r *cmuxcontract.PreflightRequest) { r.ExpectedBuild = "cmux 0.64.10" }, want: "identity is incomplete"},
		{name: "empty socket", mutate: func(r *cmuxcontract.PreflightRequest) { r.SocketPath = "" }, want: "identity is incomplete"},
		{name: "NUL socket", mutate: func(r *cmuxcontract.PreflightRequest) { r.SocketPath = "/tmp/a\x00b" }, want: "identity is incomplete"},
		{name: "non-uuid window", mutate: func(r *cmuxcontract.PreflightRequest) { r.WindowID = "window-1" }, want: "identity is incomplete"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := test.client
			if client.Runner == nil && client.ObserveEndpoint == nil {
				client = Client{Runner: &queueRunner{t: t}, ObserveEndpoint: observer}
			}
			request := preflightRequestFixture()
			if test.mutate != nil {
				test.mutate(&request)
			}
			_, err := client.Preflight(t.Context(), request)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want %q", err, test.want)
			}
			if observed != 0 {
				t.Fatalf("endpoint observed %d times for invalid request", observed)
			}
		})
	}
}

func TestPreflightEndpointObservationFailuresAndRestartAbort(t *testing.T) {
	t.Run("first observation fails before any command", func(t *testing.T) {
		runner := &queueRunner{t: t}
		boom := errors.New("socket gone")
		client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) {
			return issueops.IssueOpsHandoffDeliveryEndpointIncarnation{}, boom
		}}
		if _, err := client.Preflight(t.Context(), preflightRequestFixture()); !errors.Is(err, boom) || len(runner.seen) != 0 {
			t.Fatalf("err=%v commands=%d", err, len(runner.seen))
		}
	})
	t.Run("second observation fails", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: okPreflightSteps()}
		boom := errors.New("socket gone")
		calls := 0
		client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) {
			calls++
			if calls == 2 {
				return issueops.IssueOpsHandoffDeliveryEndpointIncarnation{}, boom
			}
			return endpointFixture(), nil
		}}
		if _, err := client.Preflight(t.Context(), preflightRequestFixture()); !errors.Is(err, boom) {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("endpoint replaced during preflight", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: okPreflightSteps()}
		calls := 0
		client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) {
			calls++
			endpoint := endpointFixture()
			endpoint.Inode += uint64(calls)
			return endpoint, nil
		}}
		_, err := client.Preflight(t.Context(), preflightRequestFixture())
		if err == nil || !strings.Contains(err.Error(), "incarnation changed during preflight") {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestPreflightCommandFailuresNameTheFailingStep(t *testing.T) {
	boom := errors.New("exit 1")
	steps := okPreflightSteps()
	tests := []struct {
		name  string
		steps []runnerStep
		want  string
	}{
		{name: "version command fails", steps: []runnerStep{{args: []string{"version"}, err: boom}}, want: "version preflight failed"},
		{name: "ping answers wrong", steps: []runnerStep{steps[0], {args: []string{"ping"}, stdout: "NOPE\n"}}, want: "ping preflight failed"},
		{name: "capabilities command fails", steps: []runnerStep{steps[0], steps[1], {args: []string{"capabilities"}, err: boom}}, want: "capabilities preflight failed"},
		{name: "wrong protocol", steps: []runnerStep{steps[0], steps[1], {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), "cmux-socket", "other", 1)}}, want: "identity is incomplete"},
		{name: "wrong protocol version", steps: []runnerStep{steps[0], steps[1], {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), `"version":2`, `"version":3`, 1)}}, want: "identity is incomplete"},
		{name: "capabilities for other socket", steps: []runnerStep{steps[0], steps[1], {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), socketPath, "/private/tmp/other.sock", 1)}}, want: "identity is incomplete"},
		{name: "missing access mode", steps: []runnerStep{steps[0], steps[1], {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), `"passwordless"`, `""`, 1)}}, want: "identity is incomplete"},
		{name: "duplicate method", steps: []runnerStep{steps[0], steps[1], {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), `"system.ping"`, `"system.ping","system.ping"`, 1)}}, want: "duplicate method"},
		{name: "identify command fails", steps: []runnerStep{steps[0], steps[1], steps[2], {args: steps[3].args, err: boom}}, want: "identify preflight failed"},
		{name: "identify socket differs", steps: []runnerStep{steps[0], steps[1], steps[2], {args: steps[3].args, stdout: strings.Replace(identifyFixture(testWindow, "", ""), socketPath, "/private/tmp/other.sock", 1)}}, want: "window identity mismatch"},
		{name: "identify has no focused window", steps: []runnerStep{steps[0], steps[1], steps[2], {args: steps[3].args, stdout: `{"socket_path":"` + socketPath + `","caller":null}`}}, want: "window identity mismatch"},
		{name: "identify leaks caller", steps: []runnerStep{steps[0], steps[1], steps[2], {args: steps[3].args, stdout: identifyFixture(testWindow, testWorkspace, testSurface)}}, want: "unexpectedly returned caller"},
		{name: "identify malformed", steps: []runnerStep{steps[0], steps[1], steps[2], {args: steps[3].args, stdout: "not json"}}, want: "identify response is malformed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := Client{Runner: &queueRunner{t: t, steps: test.steps}, ObserveEndpoint: stableEndpointObserver, UID: 501}
			_, err := client.Preflight(t.Context(), preflightRequestFixture())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want %q", err, test.want)
			}
		})
	}
}

func createSteps(name string) []runnerStep {
	return []runnerStep{
		{args: []string{"new-workspace", "--name", name, "--cwd", "/repo/worktree", "--window", testWindow, "--focus", "false"}, stdout: "OK workspace:7\n"},
		{args: []string{"--json", "--id-format", "both", "list-workspaces", "--window", testWindow}, stdout: `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"` + name + `"}]}`},
		{args: []string{"--json", "--id-format", "uuids", "list-panes", "--window", testWindow, "--workspace", testWorkspace}, stdout: `{"panes":[{"id":"` + testPane + `","surface_count":1}]}`},
		{args: []string{"--json", "--id-format", "uuids", "list-pane-surfaces", "--window", testWindow, "--workspace", testWorkspace, "--pane", testPane}, stdout: `{"surfaces":[{"id":"` + testSurface + `"}]}`},
		{args: []string{"--id-format", "uuids", "identify", "--window", testWindow, "--workspace", testWorkspace, "--surface", testSurface}, stdout: identifyFixture(testWindow, testWorkspace, testSurface)},
	}
}

func TestCreateWorkspaceRejectsInvalidRequestWithoutTouchingCmux(t *testing.T) {
	for name, mutate := range map[string]func(*cmuxcontract.CreateRequest){
		"non-uuid window": func(r *cmuxcontract.CreateRequest) { r.Preflight.WindowID = "w" },
		"relative cwd":    func(r *cmuxcontract.CreateRequest) { r.CWD = "repo" },
		"unclean cwd":     func(r *cmuxcontract.CreateRequest) { r.CWD = "/repo/../repo" },
		"blank attempt":   func(r *cmuxcontract.CreateRequest) { r.AttemptID = "  " },
	} {
		t.Run(name, func(t *testing.T) {
			runner := &queueRunner{t: t}
			client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
			request := cmuxcontract.CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"}
			mutate(&request)
			_, err := client.CreateWorkspace(t.Context(), request)
			var mutation *cmuxport.MutationError
			if err == nil || errors.As(err, &mutation) || len(runner.seen) != 0 {
				t.Fatalf("err=%v commands=%d", err, len(runner.seen))
			}
		})
	}
}

func TestCreateWorkspaceRefusesBeforeCreateWhenEndpointAlreadyChanged(t *testing.T) {
	runner := &queueRunner{t: t}
	moved := endpointFixture()
	moved.Inode++
	client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) { return moved, nil }}
	_, err := client.CreateWorkspace(t.Context(), cmuxcontract.CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"})
	var mutation *cmuxport.MutationError
	if err == nil || errors.As(err, &mutation) || len(runner.seen) != 0 {
		t.Fatalf("err=%v commands=%d", err, len(runner.seen))
	}
}

func TestCreateWorkspaceFailureThatNeverInvokedCmuxIsNotAmbiguous(t *testing.T) {
	name := workspaceName("attempt-1")
	step := createSteps(name)[0]
	step.stdout = ""
	step.err = errors.New("fork failed")
	step.invoked = false
	client := Client{Runner: &queueRunner{t: t, steps: []runnerStep{step}}, ObserveEndpoint: stableEndpointObserver, UID: 501}
	_, err := client.CreateWorkspace(t.Context(), cmuxcontract.CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"})
	var mutation *cmuxport.MutationError
	if !errors.As(err, &mutation) || mutation.Ambiguous || mutation.Phase != "workspace_create" {
		t.Fatalf("err=%v mutation=%+v", err, mutation)
	}
}

func TestCreateWorkspaceTargetResolutionFailuresAreAmbiguousAndStopImmediately(t *testing.T) {
	name := workspaceName("attempt-1")
	otherUUID := "55555555-5555-4555-8555-555555555555"
	tests := []struct {
		name      string
		replace   map[int]runnerStep
		wantCalls int
		wantErr   string
		phase     string
	}{
		{name: "malformed create response", replace: map[int]runnerStep{0: {stdout: "workspace:7\n"}}, wantCalls: 1, wantErr: "malformed create response", phase: "workspace_create"},
		{name: "create response with extra field", replace: map[int]runnerStep{0: {stdout: "OK workspace:7 extra\n"}}, wantCalls: 1, wantErr: "malformed create response", phase: "workspace_create"},
		{name: "create response bad ref", replace: map[int]runnerStep{0: {stdout: "OK workspace:x\n"}}, wantCalls: 1, wantErr: "malformed create response", phase: "workspace_create"},
		{name: "list workspaces fails", replace: map[int]runnerStep{1: {err: errors.New("boom"), invoked: true}}, wantCalls: 2, wantErr: "boom", phase: "target_resolve"},
		{name: "list workspaces malformed", replace: map[int]runnerStep{1: {stdout: `{"workspaces":`}}, wantCalls: 2, wantErr: "malformed workspace list", phase: "target_resolve"},
		{name: "duplicate workspace identity", replace: map[int]runnerStep{1: {stdout: `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"` + name + `"},{"id":"` + otherUUID + `","ref":"workspace:7","title":"` + name + `"}]}`}}, wantCalls: 2, wantErr: "duplicate created workspace identity", phase: "target_resolve"},
		{name: "title mismatch", replace: map[int]runnerStep{1: {stdout: `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"other"}]}`}}, wantCalls: 2, wantErr: "identity is unavailable", phase: "target_resolve"},
		{name: "non uuid workspace id", replace: map[int]runnerStep{1: {stdout: `{"workspaces":[{"id":"workspace:7","ref":"workspace:7","title":"` + name + `"}]}`}}, wantCalls: 2, wantErr: "identity is unavailable", phase: "target_resolve"},
		{name: "list panes fails", replace: map[int]runnerStep{2: {err: errors.New("boom"), invoked: true}}, wantCalls: 3, wantErr: "boom", phase: "target_resolve"},
		{name: "two panes", replace: map[int]runnerStep{2: {stdout: `{"panes":[{"id":"` + testPane + `","surface_count":1},{"id":"` + otherUUID + `","surface_count":1}]}`}}, wantCalls: 3, wantErr: "pane identity is incomplete", phase: "target_resolve"},
		{name: "pane with two surfaces", replace: map[int]runnerStep{2: {stdout: `{"panes":[{"id":"` + testPane + `","surface_count":2}]}`}}, wantCalls: 3, wantErr: "pane identity is incomplete", phase: "target_resolve"},
		{name: "pane id not uuid", replace: map[int]runnerStep{2: {stdout: `{"panes":[{"id":"pane:1","surface_count":1}]}`}}, wantCalls: 3, wantErr: "pane identity is incomplete", phase: "target_resolve"},
		{name: "list surfaces fails", replace: map[int]runnerStep{3: {err: errors.New("boom"), invoked: true}}, wantCalls: 4, wantErr: "boom", phase: "target_resolve"},
		{name: "no surfaces", replace: map[int]runnerStep{3: {stdout: `{"surfaces":[]}`}}, wantCalls: 4, wantErr: "surface identity is incomplete", phase: "target_resolve"},
		{name: "surface id not uuid", replace: map[int]runnerStep{3: {stdout: `{"surfaces":[{"id":"surface:1"}]}`}}, wantCalls: 4, wantErr: "surface identity is incomplete", phase: "target_resolve"},
		{name: "identify fails", replace: map[int]runnerStep{4: {err: errors.New("boom"), invoked: true}}, wantCalls: 5, wantErr: "boom", phase: "target_resolve"},
		{name: "identify caller is other surface", replace: map[int]runnerStep{4: {stdout: identifyFixture(testWindow, testWorkspace, otherUUID)}}, wantCalls: 5, wantErr: "target identity mismatch", phase: "target_resolve"},
		{name: "identify has no caller", replace: map[int]runnerStep{4: {stdout: identifyFixture(testWindow, "", "")}}, wantCalls: 5, wantErr: "target identity mismatch", phase: "target_resolve"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			steps := createSteps(name)
			for index, replacement := range test.replace {
				replacement.args = steps[index].args
				steps[index] = replacement
			}
			runner := &queueRunner{t: t, steps: steps}
			client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
			_, err := client.CreateWorkspace(t.Context(), cmuxcontract.CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"})
			var mutation *cmuxport.MutationError
			if !errors.As(err, &mutation) || !mutation.Ambiguous || mutation.Phase != test.phase || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("err=%v mutation=%+v", err, mutation)
			}
			if len(runner.seen) != test.wantCalls {
				t.Fatalf("commands=%d want %d (must not retry or continue past a failed step)", len(runner.seen), test.wantCalls)
			}
		})
	}
}

func TestCreateWorkspaceAcceptsUUIDHandleAndRecordsTimings(t *testing.T) {
	name := workspaceName("attempt-1")
	steps := createSteps(name)
	steps[0].stdout = "OK " + testWorkspace + "\n"
	steps[1].stdout = `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"` + name + `"}]}`
	client := Client{Runner: &queueRunner{t: t, steps: steps}, ObserveEndpoint: stableEndpointObserver, UID: 501}
	created, err := client.CreateWorkspace(t.Context(), cmuxcontract.CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"})
	if err != nil {
		t.Fatal(err)
	}
	if created.WorkspaceID != testWorkspace || created.CreateMS == 0 || created.ResolveMS == 0 {
		t.Fatalf("created=%+v", created)
	}
}

func TestSendRejectsIncompleteTargetsAndEndpointChangesBeforeTyping(t *testing.T) {
	t.Run("incomplete target or blank command", func(t *testing.T) {
		for name, mutate := range map[string]func(*cmuxcontract.SendRequest){
			"window":    func(r *cmuxcontract.SendRequest) { r.Created.WindowID = "" },
			"workspace": func(r *cmuxcontract.SendRequest) { r.Created.WorkspaceID = "ws" },
			"surface":   func(r *cmuxcontract.SendRequest) { r.Created.SurfaceID = "" },
			"command":   func(r *cmuxcontract.SendRequest) { r.Command = " \t" },
		} {
			t.Run(name, func(t *testing.T) {
				runner := &queueRunner{t: t}
				client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
				request := cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"}
				mutate(&request)
				_, err := client.Send(t.Context(), request)
				var mutation *cmuxport.MutationError
				if err == nil || errors.As(err, &mutation) || len(runner.seen) != 0 {
					t.Fatalf("err=%v commands=%d", err, len(runner.seen))
				}
			})
		}
	})
	t.Run("endpoint changed before send", func(t *testing.T) {
		runner := &queueRunner{t: t}
		moved := endpointFixture()
		moved.Inode++
		client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) { return moved, nil }}
		_, err := client.Send(t.Context(), cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"})
		var mutation *cmuxport.MutationError
		if err == nil || errors.As(err, &mutation) || len(runner.seen) != 0 {
			t.Fatalf("err=%v commands=%d", err, len(runner.seen))
		}
	})
}

func TestSendClassifiesFailuresAndRequiresQueuedFieldAndStableEndpoint(t *testing.T) {
	t.Run("command failed before cmux was invoked", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: []runnerStep{{args: sendArgs("run\\n"), err: errors.New("fork failed")}}}
		client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
		_, err := client.Send(t.Context(), cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"})
		var mutation *cmuxport.MutationError
		if !errors.As(err, &mutation) || mutation.Ambiguous || mutation.Phase != "input_send" {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("malformed receipt", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: []runnerStep{{args: sendArgs("run\\n"), stdout: `{"window_id":`}}}
		client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
		receipt, err := client.Send(t.Context(), cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"})
		var mutation *cmuxport.MutationError
		if !errors.As(err, &mutation) || !mutation.Ambiguous || !strings.Contains(err.Error(), "malformed send response") || receipt.Accepted {
			t.Fatalf("err=%v receipt=%+v", err, receipt)
		}
	})
	t.Run("endpoint replaced after send", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: []runnerStep{{args: sendArgs("run\\n"), stdout: sendReceiptFixture(testWindow, testWorkspace, testSurface)}}}
		calls := 0
		client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) {
			calls++
			endpoint := endpointFixture()
			if calls > 1 {
				endpoint.Inode++
			}
			return endpoint, nil
		}}
		receipt, err := client.Send(t.Context(), cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"})
		var mutation *cmuxport.MutationError
		if !errors.As(err, &mutation) || !mutation.Ambiguous || receipt.Accepted {
			t.Fatalf("err=%v receipt=%+v", err, receipt)
		}
	})
	t.Run("accepted receipt records timing", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: []runnerStep{{args: sendArgs("run\\n"), stdout: sendReceiptFixture(testWindow, testWorkspace, testSurface)}}}
		client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
		receipt, err := client.Send(t.Context(), cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"})
		if err != nil || !receipt.Accepted || receipt.SendMS == 0 {
			t.Fatalf("err=%v receipt=%+v", err, receipt)
		}
	})
}

func TestDecodeStrictRejectsAmbiguousOrTrailingJSON(t *testing.T) {
	type target struct {
		A int   `json:"a"`
		B []int `json:"b"`
	}
	var ok target
	if err := decodeStrict([]byte(` {"a":1,"b":[1,2,{"x":1}]} `), &map[string]any{}); err != nil {
		t.Fatalf("valid nested JSON rejected: %v", err)
	}
	if err := decodeStrict([]byte(`{"a":1,"b":[1,2]}`), &ok); err != nil || ok.A != 1 || len(ok.B) != 2 {
		t.Fatalf("decoded=%+v err=%v", ok, err)
	}
	for name, input := range map[string]string{
		"empty":                "",
		"trailing value":       `{"a":1} {"a":2}`,
		"trailing garbage":     `{"a":1} x`,
		"duplicate key":        `{"a":1,"a":2}`,
		"nested duplicate key": `{"b":[{"k":1,"k":2}]}`,
		"truncated object":     `{"a":1`,
		"truncated array":      `{"b":[1,2`,
		"unexpected closer":    `]`,
		"wrong destination":    `[1,2]`,
	} {
		t.Run(name, func(t *testing.T) {
			var dst target
			if err := decodeStrict([]byte(input), &dst); err == nil {
				t.Fatalf("accepted %q", input)
			}
		})
	}
}

func TestParseCreatedWorkspaceHandleAcceptsOnlyOKWithRefOrUUID(t *testing.T) {
	for input, want := range map[string]string{
		"OK workspace:12\n":   "workspace:12",
		"OK " + testWorkspace: testWorkspace,
	} {
		if got, err := parseCreatedWorkspaceHandle([]byte(input)); err != nil || got != want {
			t.Errorf("%q => %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"", "OK", "ERR workspace:1", "OK workspace:", "OK workspace:1x", "OK workspace:1 workspace:2", "ok workspace:1"} {
		if got, err := parseCreatedWorkspaceHandle([]byte(input)); err == nil {
			t.Errorf("accepted %q => %q", input, got)
		}
	}
}

func TestElapsedMSNeverReportsZero(t *testing.T) {
	if got := elapsedMS(time.Now()); got != 1 {
		t.Fatalf("sub-millisecond elapsed=%d want clamp to 1", got)
	}
	if got := elapsedMS(time.Now().Add(-2 * time.Second)); got < 2000 {
		t.Fatalf("elapsed=%d want >=2000", got)
	}
}

func TestExecRunnerCapturesOutputAndFiltersAmbientCmuxScope(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "ambient-workspace")
	t.Setenv("CMUX_SURFACE_ID", "ambient-surface")
	t.Setenv("CMUX_SOCKET", "/ambient.sock")
	t.Setenv("CMUX_SOCKET_PATH", "/ambient-path.sock")
	t.Setenv("ISSUEOPS_RUNNER_PASSTHROUGH", "kept")
	output, err := ExecRunner{}.Run(t.Context(), CommandRequest{
		Executable: "/bin/sh",
		Args:       []string{"-c", `printf '%s|%s|%s|%s|%s' "${CMUX_SOCKET_PATH-unset}" "${CMUX_WORKSPACE_ID-unset}" "${CMUX_SURFACE_ID-unset}" "${CMUX_SOCKET-unset}" "$ISSUEOPS_RUNNER_PASSTHROUGH"; printf warn >&2`},
		Env:        map[string]string{"CMUX_SOCKET_PATH": "/explicit.sock", "CMUX_SOCKET": ""},
		Timeout:    5 * time.Second,
	})
	if err != nil || !output.Invoked {
		t.Fatalf("output=%+v err=%v", output, err)
	}
	if got := string(output.Stdout); got != "/explicit.sock|unset|unset|unset|kept" {
		t.Fatalf("stdout=%q", got)
	}
	if string(output.Stderr) != "warn" {
		t.Fatalf("stderr=%q", output.Stderr)
	}
}

func TestExecRunnerReportsFailureKindsWithInvocationState(t *testing.T) {
	t.Run("missing executable was never invoked", func(t *testing.T) {
		output, err := ExecRunner{}.Run(t.Context(), CommandRequest{Executable: "/nonexistent/cmux", Timeout: time.Second})
		if err == nil || output.Invoked {
			t.Fatalf("output=%+v err=%v", output, err)
		}
	})
	t.Run("non-zero exit keeps stderr in error and marks invoked", func(t *testing.T) {
		output, err := ExecRunner{}.Run(t.Context(), CommandRequest{Executable: "/bin/sh", Args: []string{"-c", "printf 'denied \\n' >&2; printf partial; exit 3"}, Timeout: 5 * time.Second})
		if err == nil || !output.Invoked || !strings.Contains(err.Error(), "denied") || !strings.Contains(err.Error(), "exit status 3") || string(output.Stdout) != "partial" {
			t.Fatalf("output=%+v err=%v", output, err)
		}
	})
	t.Run("deadline kills a hung command", func(t *testing.T) {
		started := time.Now()
		output, err := ExecRunner{}.Run(t.Context(), CommandRequest{Executable: "/bin/sh", Args: []string{"-c", "exec sleep 30"}, Timeout: 100 * time.Millisecond})
		if err == nil || !output.Invoked || time.Since(started) > 10*time.Second {
			t.Fatalf("output=%+v err=%v elapsed=%s", output, err, time.Since(started))
		}
	})
	t.Run("deadline kills a grandchild that holds the output pipe", func(t *testing.T) {
		marker := filepath.Join(t.TempDir(), "survived")
		started := time.Now()
		// The background sleep inherits stdout; without a group kill and
		// WaitDelay, Wait blocks until it exits and then it writes the marker.
		script := "(sleep 1; touch " + marker + ") & sleep 30"
		output, err := ExecRunner{}.Run(t.Context(), CommandRequest{Executable: "/bin/sh", Args: []string{"-c", script}, Timeout: 100 * time.Millisecond})
		if err == nil || !output.Invoked || time.Since(started) > 2500*time.Millisecond {
			t.Fatalf("output=%+v err=%v elapsed=%s", output, err, time.Since(started))
		}
		time.Sleep(1500 * time.Millisecond)
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Fatal("the grandchild survived the deadline")
		}
	})
}

func TestCommandEnvironmentDropsAmbientCmuxScopeAndAppliesOnlyNonEmptyOverrides(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "ambient")
	t.Setenv("CMUX_SOCKET", "ambient")
	t.Setenv("ISSUEOPS_ENV_PROBE", "x=y")
	environment := commandEnvironment(map[string]string{"CMUX_SOCKET_PATH": "/s.sock", "CMUX_SURFACE_ID": "", "UNLISTED": "ignored"})
	seen := map[string]string{}
	for _, entry := range environment {
		name, value, _ := strings.Cut(entry, "=")
		seen[name] = value
	}
	if seen["CMUX_SOCKET_PATH"] != "/s.sock" || seen["ISSUEOPS_ENV_PROBE"] != "x=y" {
		t.Fatalf("override or passthrough missing: %v", seen)
	}
	for _, name := range []string{"CMUX_WORKSPACE_ID", "CMUX_SOCKET", "CMUX_SURFACE_ID", "UNLISTED"} {
		if _, present := seen[name]; present {
			t.Errorf("%s leaked into command environment", name)
		}
	}
}

func TestCreateAndSendPropagateEndpointObservationFailureWithoutRunningCommands(t *testing.T) {
	boom := errors.New("socket unavailable")
	runner := &queueRunner{t: t}
	client := Client{Runner: runner, UID: 501, ObserveEndpoint: func(string, int) (issueops.IssueOpsHandoffDeliveryEndpointIncarnation, error) {
		return issueops.IssueOpsHandoffDeliveryEndpointIncarnation{}, boom
	}}
	if _, err := client.CreateWorkspace(t.Context(), cmuxcontract.CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"}); !errors.Is(err, boom) {
		t.Fatalf("create err=%v", err)
	}
	if _, err := client.Send(t.Context(), cmuxcontract.SendRequest{Created: createdFixture(), Command: "run"}); !errors.Is(err, boom) {
		t.Fatalf("send err=%v", err)
	}
	if len(runner.seen) != 0 {
		t.Fatalf("commands ran despite unobservable endpoint: %d", len(runner.seen))
	}
}

func TestExecRunnerCapsOutputAtOneMiBAndFails(t *testing.T) {
	for name, script := range map[string]string{
		"stdout": "exec head -c 1100000 /dev/zero",
		"stderr": "exec head -c 1100000 /dev/zero >&2",
	} {
		t.Run(name, func(t *testing.T) {
			output, err := ExecRunner{}.Run(t.Context(), CommandRequest{Executable: "/bin/sh", Args: []string{"-c", script}, Timeout: 10 * time.Second})
			if err == nil || !strings.Contains(err.Error(), "exceeded 1 MiB") || !output.Invoked {
				t.Fatalf("invoked=%v err=%v", output.Invoked, err)
			}
			captured := output.Stdout
			if name == "stderr" {
				captured = output.Stderr
			}
			if len(captured) != 1<<20 {
				t.Fatalf("captured %d bytes want %d", len(captured), 1<<20)
			}
		})
	}
}
