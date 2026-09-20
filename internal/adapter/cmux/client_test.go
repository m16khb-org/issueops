package cmux

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	testWindow    = "11111111-1111-4111-8111-111111111111"
	testWorkspace = "22222222-2222-4222-8222-222222222222"
	testPane      = "33333333-3333-4333-8333-333333333333"
	testSurface   = "44444444-4444-4444-8444-444444444444"
	cmuxPath      = "/Applications/cmux.app/Contents/Resources/bin/cmux"
	socketPath    = "/private/tmp/cmux-test.sock"
)

func TestPreflightUsesBoundedReadOnlyCmuxCommandsAndExactObservedIdentity(t *testing.T) {
	runner := &queueRunner{t: t, steps: []runnerStep{
		{args: []string{"version"}, stdout: "cmux 0.64.10 (90) [fafa50702]\n"},
		{args: []string{"ping"}, stdout: "PONG\n"},
		{args: []string{"capabilities"}, stdout: capabilitiesFixture()},
		{args: []string{"--id-format", "uuids", "identify", "--window", testWindow, "--no-caller"}, stdout: identifyFixture(testWindow, "", "")},
	}}
	observations := 0
	client := Client{
		Runner: runner,
		ObserveEndpoint: func(path string, uid int) (EndpointIncarnation, error) {
			observations++
			if path != socketPath || uid != 501 {
				t.Fatalf("endpoint observation path=%q uid=%d", path, uid)
			}
			return endpointFixture(), nil
		},
		UID: 501,
	}
	result, err := client.Preflight(context.Background(), PreflightRequest{
		Executable: cmuxPath, ExpectedVersion: "0.64.10", SocketPath: socketPath, WindowID: testWindow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if observations != 2 || result.Version != "0.64.10" || result.WindowID != testWindow || !SameEndpoint(result.Endpoint, endpointFixture()) {
		t.Fatalf("preflight=%+v observations=%d", result, observations)
	}
	for _, request := range runner.seen {
		if request.Executable != cmuxPath || request.Timeout <= 0 || request.Timeout > 5*time.Second {
			t.Fatalf("unbounded or wrong command: %+v", request)
		}
		if len(request.Args) < 3 || request.Args[0] != "--socket" || request.Args[1] != socketPath {
			t.Fatalf("cmux command did not pin the socket as an explicit CLI target: %q", request.Args)
		}
		if request.Env["CMUX_SOCKET_PATH"] != socketPath || request.Env["CMUX_SOCKET"] != "" || request.Env["CMUX_WORKSPACE_ID"] != "" || request.Env["CMUX_SURFACE_ID"] != "" {
			t.Fatalf("cmux command inherited implicit target: %+v", request.Env)
		}
	}
}

func TestPreflightFailsClosedOnMalformedOrIncompleteCmuxEvidence(t *testing.T) {
	tests := []struct {
		name  string
		steps []runnerStep
		want  string
	}{
		{name: "version mismatch", steps: []runnerStep{{args: []string{"version"}, stdout: "cmux 0.64.9\n"}}, want: "version mismatch"},
		{name: "ping denied", steps: []runnerStep{{args: []string{"version"}, stdout: "cmux 0.64.10 (90) [fafa50702]\n"}, {args: []string{"ping"}, err: errors.New("permission denied")}}, want: "ping"},
		{name: "malformed capabilities", steps: []runnerStep{{args: []string{"version"}, stdout: "cmux 0.64.10 (90) [fafa50702]\n"}, {args: []string{"ping"}, stdout: "PONG\n"}, {args: []string{"capabilities"}, stdout: "{"}}, want: "capabilities"},
		{name: "duplicate capabilities field", steps: []runnerStep{{args: []string{"version"}, stdout: "cmux 0.64.10 (90) [fafa50702]\n"}, {args: []string{"ping"}, stdout: "PONG\n"}, {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), "{", `{"protocol":"other",`, 1)}}, want: "duplicate JSON key"},
		{name: "missing method", steps: []runnerStep{{args: []string{"version"}, stdout: "cmux 0.64.10 (90) [fafa50702]\n"}, {args: []string{"ping"}, stdout: "PONG\n"}, {args: []string{"capabilities"}, stdout: strings.Replace(capabilitiesFixture(), `,"surface.send_text"`, "", 1)}}, want: "required capability"},
		{name: "target mismatch", steps: []runnerStep{{args: []string{"version"}, stdout: "cmux 0.64.10 (90) [fafa50702]\n"}, {args: []string{"ping"}, stdout: "PONG\n"}, {args: []string{"capabilities"}, stdout: capabilitiesFixture()}, {args: []string{"--id-format", "uuids", "identify", "--window", testWindow, "--no-caller"}, stdout: identifyFixture("99999999-9999-4999-8999-999999999999", "", "")}}, want: "window identity mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runner := &queueRunner{t: t, steps: test.steps}
			client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
			_, err := client.Preflight(context.Background(), PreflightRequest{Executable: cmuxPath, ExpectedVersion: "0.64.10", SocketPath: socketPath, WindowID: testWindow})
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v want=%q", err, test.want)
			}
		})
	}
}

func TestCreateWorkspaceResolvesOneExactWorkspaceSurfaceWithoutFocusDefaults(t *testing.T) {
	name := workspaceName("attempt-1")
	runner := &queueRunner{t: t, steps: []runnerStep{
		{args: []string{"new-workspace", "--name", name, "--cwd", "/repo/worktree", "--window", testWindow, "--focus", "false"}, stdout: "OK workspace:7\n"},
		{args: []string{"--json", "--id-format", "both", "list-workspaces", "--window", testWindow}, stdout: `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"` + name + `"}]}`},
		{args: []string{"--json", "--id-format", "uuids", "list-panes", "--window", testWindow, "--workspace", testWorkspace}, stdout: `{"panes":[{"id":"` + testPane + `","surface_count":1}]}`},
		{args: []string{"--json", "--id-format", "uuids", "list-pane-surfaces", "--window", testWindow, "--workspace", testWorkspace, "--pane", testPane}, stdout: `{"surfaces":[{"id":"` + testSurface + `"}]}`},
		{args: []string{"--id-format", "uuids", "identify", "--window", testWindow, "--workspace", testWorkspace, "--surface", testSurface}, stdout: identifyFixture(testWindow, testWorkspace, testSurface)},
	}}
	client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
	created, err := client.CreateWorkspace(context.Background(), CreateRequest{
		Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.WindowID != testWindow || created.WorkspaceID != testWorkspace || created.PaneID != testPane || created.SurfaceID != testSurface || created.CWD != "/repo/worktree" {
		t.Fatalf("created=%+v", created)
	}
}

func TestCreateWorkspaceRejectsReturnedWorkspaceRefMismatchAsAmbiguous(t *testing.T) {
	name := workspaceName("attempt-1")
	runner := &queueRunner{t: t, steps: []runnerStep{
		{args: []string{"new-workspace", "--name", name, "--cwd", "/repo/worktree", "--window", testWindow, "--focus", "false"}, stdout: "OK workspace:9\n"},
		{args: []string{"--json", "--id-format", "both", "list-workspaces", "--window", testWindow}, stdout: `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"` + name + `"}]}`},
	}}
	client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
	_, err := client.CreateWorkspace(context.Background(), CreateRequest{
		Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree",
	})
	var mutation *MutationError
	if !errors.As(err, &mutation) || !mutation.Ambiguous || mutation.Phase != "target_resolve" || len(runner.seen) != 2 {
		t.Fatalf("returned workspace mismatch error=%v calls=%d", err, len(runner.seen))
	}
}

func TestSendTargetsExactSurfaceOnceAndTreatsResponseLossAsAmbiguous(t *testing.T) {
	for _, test := range []struct {
		name      string
		step      runnerStep
		ambiguous bool
	}{
		{name: "accepted", step: runnerStep{args: sendArgs("/bin/sh '/tmp/io-cmux/launch.sh'\\n"), stdout: sendReceiptFixture(testWindow, testWorkspace, testSurface)}},
		{name: "target mismatch", step: runnerStep{args: sendArgs("/bin/sh '/tmp/io-cmux/launch.sh'\\n"), stdout: sendReceiptFixture(testWindow, testWorkspace, "99999999-9999-4999-8999-999999999999")}, ambiguous: true},
		{name: "incomplete receipt", step: runnerStep{args: sendArgs("/bin/sh '/tmp/io-cmux/launch.sh'\\n"), stdout: `{"window_id":"` + testWindow + `","workspace_id":"` + testWorkspace + `","surface_id":"` + testSurface + `"}`}, ambiguous: true},
		{name: "accepted response lost", step: runnerStep{args: sendArgs("/bin/sh '/tmp/io-cmux/launch.sh'\\n"), invoked: true, err: errors.New("EOF")}, ambiguous: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &queueRunner{t: t, steps: []runnerStep{test.step}}
			client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
			_, err := client.Send(context.Background(), SendRequest{Created: createdFixture(), Command: "/bin/sh '/tmp/io-cmux/launch.sh'"})
			if test.ambiguous {
				var mutation *MutationError
				if !errors.As(err, &mutation) || !mutation.Ambiguous || mutation.Phase != "input_send" || len(runner.seen) != 1 {
					t.Fatalf("ambiguous error=%v calls=%d", err, len(runner.seen))
				}
				return
			}
			if err != nil || len(runner.seen) != 1 {
				t.Fatalf("send error=%v calls=%d", err, len(runner.seen))
			}
		})
	}
}

func TestCreateWorkspaceResponseLossAndEndpointRestartNeverRetry(t *testing.T) {
	t.Run("create response loss", func(t *testing.T) {
		runner := &queueRunner{t: t, steps: []runnerStep{{args: []string{"new-workspace", "--name", workspaceName("attempt-1"), "--cwd", "/repo/worktree", "--window", testWindow, "--focus", "false"}, invoked: true, err: errors.New("EOF")}}}
		client := Client{Runner: runner, ObserveEndpoint: stableEndpointObserver, UID: 501}
		_, err := client.CreateWorkspace(context.Background(), CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"})
		var mutation *MutationError
		if !errors.As(err, &mutation) || !mutation.Ambiguous || len(runner.seen) != 1 {
			t.Fatalf("error=%v calls=%d", err, len(runner.seen))
		}
	})

	t.Run("endpoint restart after create", func(t *testing.T) {
		name := workspaceName("attempt-1")
		runner := &queueRunner{t: t, steps: []runnerStep{
			{args: []string{"new-workspace", "--name", name, "--cwd", "/repo/worktree", "--window", testWindow, "--focus", "false"}, stdout: "OK workspace:7\n"},
			{args: []string{"--json", "--id-format", "both", "list-workspaces", "--window", testWindow}, stdout: `{"workspaces":[{"id":"` + testWorkspace + `","ref":"workspace:7","title":"` + name + `"}]}`},
			{args: []string{"--json", "--id-format", "uuids", "list-panes", "--window", testWindow, "--workspace", testWorkspace}, stdout: `{"panes":[{"id":"` + testPane + `","surface_count":1}]}`},
			{args: []string{"--json", "--id-format", "uuids", "list-pane-surfaces", "--window", testWindow, "--workspace", testWorkspace, "--pane", testPane}, stdout: `{"surfaces":[{"id":"` + testSurface + `"}]}`},
			{args: []string{"--id-format", "uuids", "identify", "--window", testWindow, "--workspace", testWorkspace, "--surface", testSurface}, stdout: identifyFixture(testWindow, testWorkspace, testSurface)},
		}}
		calls := 0
		observer := func(string, int) (EndpointIncarnation, error) {
			calls++
			endpoint := endpointFixture()
			if calls > 1 {
				endpoint.Inode++
			}
			return endpoint, nil
		}
		client := Client{Runner: runner, ObserveEndpoint: observer, UID: 501}
		_, err := client.CreateWorkspace(context.Background(), CreateRequest{Preflight: preflightFixture(), AttemptID: "attempt-1", CWD: "/repo/worktree"})
		var mutation *MutationError
		if !errors.As(err, &mutation) || !mutation.Ambiguous || !strings.Contains(err.Error(), "endpoint incarnation changed") {
			t.Fatalf("error=%v", err)
		}
	})
}

type runnerStep struct {
	args    []string
	stdout  string
	stderr  string
	invoked bool
	err     error
}

type queueRunner struct {
	t     *testing.T
	steps []runnerStep
	seen  []CommandRequest
}

func (runner *queueRunner) Run(_ context.Context, request CommandRequest) (CommandOutput, error) {
	runner.t.Helper()
	runner.seen = append(runner.seen, request)
	if len(runner.steps) == 0 {
		runner.t.Fatalf("unexpected command: %+v", request)
	}
	step := runner.steps[0]
	runner.steps = runner.steps[1:]
	wantArgs := append([]string{"--socket", socketPath}, step.args...)
	if !reflect.DeepEqual(request.Args, wantArgs) {
		runner.t.Fatalf("args=%q want=%q", request.Args, wantArgs)
	}
	return CommandOutput{Stdout: []byte(step.stdout), Stderr: []byte(step.stderr), Invoked: step.invoked || step.err == nil}, step.err
}

func stableEndpointObserver(string, int) (EndpointIncarnation, error) { return endpointFixture(), nil }

func preflightFixture() PreflightResult {
	return PreflightResult{Executable: cmuxPath, Version: "0.64.10", SocketPath: socketPath, WindowID: testWindow, Endpoint: endpointFixture()}
}

func createdFixture() CreatedWorkspace {
	return CreatedWorkspace{Preflight: preflightFixture(), WindowID: testWindow, WorkspaceID: testWorkspace, PaneID: testPane, SurfaceID: testSurface, CWD: "/repo/worktree"}
}

func capabilitiesFixture() string {
	return `{"protocol":"cmux-socket","version":2,"socket_path":"` + socketPath + `","access_mode":"passwordless","methods":["system.ping","system.capabilities","system.identify","workspace.create","workspace.list","pane.list","pane.surfaces","surface.send_text"]}`
}

func identifyFixture(windowID, workspaceID, surfaceID string) string {
	caller := "null"
	if workspaceID != "" {
		caller = `{"workspace_id":"` + workspaceID + `","surface_id":"` + surfaceID + `"}`
	}
	return `{"socket_path":"` + socketPath + `","focused":{"window_id":"` + windowID + `"},"caller":` + caller + `}`
}

func sendArgs(command string) []string {
	return []string{"--json", "--id-format", "uuids", "send", "--window", testWindow, "--workspace", testWorkspace, "--surface", testSurface, "--", command}
}

func sendReceiptFixture(windowID, workspaceID, surfaceID string) string {
	return `{"window_id":"` + windowID + `","workspace_id":"` + workspaceID + `","surface_id":"` + surfaceID + `","queued":false}`
}
