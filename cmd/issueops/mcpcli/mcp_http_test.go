package mcpcli

import (
	"context"
	"errors"
	"io"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"issueops/internal/adapter/channel"
	issueopsadapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/sqlstore"
	channelapp "issueops/internal/application/channel"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestHTTPGuardEnforcesBearerHostOriginPathAndBodyLimit(t *testing.T) {
	server := startHTTPTestServer(t, testTransportServices(), nil)
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	oversized := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"pad":"` + strings.Repeat("x", httpMaxRequestBodyBytes) + `"}}`
	for _, tc := range []struct {
		name   string
		path   string
		body   string
		header map[string]string
		status int
	}{
		{name: "authorized", body: list, status: http.StatusOK},
		{name: "same origin", body: list, header: map[string]string{"Origin": "http://" + server.address}, status: http.StatusOK},
		{name: "missing bearer", body: list, header: map[string]string{"Authorization": ""}, status: http.StatusUnauthorized},
		{name: "wrong bearer", body: list, header: map[string]string{"Authorization": "Bearer not-the-installed-secret"}, status: http.StatusUnauthorized},
		{name: "foreign host", body: list, header: map[string]string{"Host": "localhost:" + strings.Split(server.address, ":")[1]}, status: http.StatusForbidden},
		{name: "foreign origin", body: list, header: map[string]string{"Origin": "http://evil.example"}, status: http.StatusForbidden},
		{name: "other path", path: "/other", body: list, status: http.StatusNotFound},
		{name: "oversized body", body: oversized, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := server
			if tc.path != "" {
				target.url = "http://" + server.address + tc.path
			}
			response, err := target.post(t.Context(), revisionInitialize, tc.body, tc.header)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, response.Body)
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.status)
			}
			if tc.status == http.StatusUnauthorized && response.Header.Get("WWW-Authenticate") == "" {
				t.Fatal("401 without WWW-Authenticate")
			}
		})
	}
	if strings.Contains(server.access.String(), server.bearer) {
		t.Fatal("access log leaked the bearer")
	}
}

func TestHTTPRegistrationRejectsUnclassifiedTool(t *testing.T) {
	deps := testTransportServices()
	deps.Catalog.Tools = append(append([]map[string]any(nil), deps.Catalog.Tools...), map[string]any{"name": "mystery_tool", "inputSchema": map[string]any{"type": "object"}})
	_, err := NewHTTPHandler(deps, HTTPOptions{Address: "127.0.0.1:47831", Bearer: strings.Repeat("A", 43)})
	if err == nil || !strings.Contains(err.Error(), "mystery_tool") {
		t.Fatalf("unclassified tool registration err = %v", err)
	}
	for _, tool := range mcpcatalog.Build().Tools {
		name, _ := tool["name"].(string)
		if mcpToolAuthorities[name].scope == toolScopeUnclassified {
			t.Fatalf("catalog tool %s is unclassified", name)
		}
	}
}

func TestHTTPWorkspaceToolsRequireCapabilityAndRejectActorFields(t *testing.T) {
	repoA, repoB := canonicalTempDir(t), canonicalTempDir(t)
	grants := map[string]fakeGrant{"/grants/a": {root: repoA, identity: fakeIdentity("session-a")}}
	var executed atomic.Int32
	deps := testTransportServices()
	deps.Execution.ExecuteExecution = func(context.Context, string, issueopscontract.ExecutionActionRequest, port.ExecutionActionDependencies) (any, error) {
		executed.Add(1)
		return map[string]any{"ok": true}, nil
	}
	server := startHTTPTestServer(t, withFakeAuthority(deps, grants, map[string][]string{"io-a": {repoA}}), nil)
	for _, tc := range []struct {
		name string
		tool string
		args map[string]any
		code string
	}{
		{name: "missing capability", tool: "atomic_commit_preflight", args: map[string]any{"path": repoA}, code: "authority_required"},
		{name: "actor mixed with capability", tool: "issueops_execution", args: map[string]any{"action": "status", "id": "io-a", "authority_file": "/grants/a", "host": "codex"}, code: "authority_invalid"},
		{name: "unmanaged credential", tool: "issueops_execution", args: map[string]any{"action": "status", "id": "io-a", "authority_file": "/grants/forged"}, code: "authority_invalid"},
		{name: "foreign workspace", tool: "command_policy_check", args: map[string]any{"authority_file": "/grants/a", "workspace_root": repoB, "cwd": repoB, "argv": []any{"git", "status"}}, code: "authority_invalid"},
		{name: "conflicting roots", tool: "project_docs_route", args: map[string]any{"authority_file": "/grants/a", "workspace_root": repoA, "repo": repoB}, code: "authority_invalid"},
		{name: "record conflict", tool: "issueops_execution", args: map[string]any{"action": "status", "id": "io-a", "authority_file": "/grants/a", "workspace_root": repoB}, code: "authority_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, isError, rpcErr, err := server.callTool(t.Context(), revisionInitialize, tc.tool, tc.args, nil)
			if err != nil || rpcErr != nil {
				t.Fatalf("err=%v rpc=%v", err, rpcErr)
			}
			if !isError || payload["error_code"] != tc.code {
				t.Fatalf("payload=%v isError=%v, want %s", payload, isError, tc.code)
			}
		})
	}
	if executed.Load() != 0 {
		t.Fatalf("rejected execution requests reached the handler %d times", executed.Load())
	}
	payload, isError, rpcErr, err := server.callTool(t.Context(), revisionInitialize, "contract_schema", map[string]any{}, nil)
	if err != nil || rpcErr != nil || isError || payload["ok"] != true {
		t.Fatalf("server-scoped tool payload=%v isError=%v rpc=%v err=%v", payload, isError, rpcErr, err)
	}
}

func TestHTTPExecutionActionsUseVerifiedCallerWithoutServerAncestry(t *testing.T) {
	repo, worktree := canonicalTempDir(t), canonicalTempDir(t)
	nested := filepath.Join(worktree, "pkg")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	identity := fakeIdentity("session-a")
	grants := map[string]fakeGrant{"/grants/a": {root: repo, identity: identity}, "/grants/w": {root: worktree, identity: identity}}
	deps := testTransportServices()
	var observed atomic.Int32
	deps.Execution.ObserveNativeProcessAncestry = func(int) ([]issueopscontract.NativeProcessReceipt, error) {
		observed.Add(1)
		return []issueopscontract.NativeProcessReceipt{{PID: 1, StartedAt: "server", Executable: "/server"}}, nil
	}
	requests := make(chan issueopscontract.ExecutionActionRequest, 1)
	deps.Execution.ExecuteExecution = func(_ context.Context, _ string, request issueopscontract.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		requests <- request
		return map[string]any{"ok": true, "action": request.Action}, nil
	}
	server := startHTTPTestServer(t, withFakeAuthority(deps, grants, map[string][]string{"io-a": {repo, worktree}}), nil)
	cases := []struct{ action, replace string }{
		{"prepare", ""}, {"status", ""}, {"claim", ""}, {"release", ""},
		{"replace", "preview"}, {"replace", "revoke"}, {"replace", "finalize-preview"}, {"replace", "finalize"}, {"replace", "reseed"},
		{"resume", ""}, {"reconcile", ""}, {"complete", ""},
	}
	for _, tc := range cases {
		t.Run(tc.action+"/"+tc.replace, func(t *testing.T) {
			args := map[string]any{"action": tc.action, "id": "io-a", "authority_file": "/grants/w", "cwd": nested, "claim_token_file": "token.json", "confirm": true}
			if tc.replace != "" {
				args["replace_action"] = tc.replace
			}
			payload, isError, rpcErr, err := server.callTool(t.Context(), revisionNew, "issueops_execution", args, nil)
			if err != nil || rpcErr != nil || isError {
				t.Fatalf("payload=%v isError=%v rpc=%v err=%v", payload, isError, rpcErr, err)
			}
			request := <-requests
			if request.Action != tc.action || request.ReplaceAction != tc.replace {
				t.Fatalf("dispatched %s/%s", request.Action, request.ReplaceAction)
			}
			if request.Actor.SessionID != "session-a" || request.Actor.ProcessAncestry != nil || *request.Actor.SessionProcess != *identity.SessionProcess {
				t.Fatalf("actor = %+v", request.Actor)
			}
			if request.CWD != nested || request.TokenFile != filepath.Join(nested, "token.json") {
				t.Fatalf("cwd=%q token_file=%q", request.CWD, request.TokenFile)
			}
		})
	}
	if observed.Load() != 0 {
		t.Fatalf("HTTP execution observed server ancestry %d times", observed.Load())
	}
}

func TestHTTPConcurrentClientsKeepCallerScopeAndTraceIsolated(t *testing.T) {
	t.Setenv("TRACEPARENT", "00-99999999999999999999999999999999-9999999999999999-01")
	repoA, repoB := canonicalTempDir(t), canonicalTempDir(t)
	grants := map[string]fakeGrant{
		"/grants/a": {root: repoA, identity: fakeIdentity("session-a")},
		"/grants/b": {root: repoB, identity: fakeIdentity("session-b")},
	}
	deps := testTransportServices()
	type traceKey struct{}
	deps.BindTrace = func(ctx context.Context, raw string) (context.Context, bool) {
		return context.WithValue(ctx, traceKey{}, raw), true
	}
	type entry struct{ id, session, cwd, grant, trace string }
	entered, release := make(chan entry, 2), make(chan struct{})
	deps.Execution.ExecuteExecution = func(ctx context.Context, _ string, request issueopscontract.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		grant, _ := ctx.Value(boundGrantKey{}).(string)
		trace, _ := ctx.Value(traceKey{}).(string)
		entered <- entry{id: request.ID, session: request.Actor.SessionID, cwd: request.CWD, grant: grant, trace: trace}
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return map[string]any{"ok": true, "id": request.ID, "session": request.Actor.SessionID}, nil
	}
	server := startHTTPTestServer(t, withFakeAuthority(deps, grants, map[string][]string{"io-a": {repoA}, "io-b": {repoB}}), nil)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	clients := map[string]struct{ id, grant, root, trace, session string }{
		"a": {"io-a", "/grants/a", repoA, "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-aaaaaaaaaaaaaaaa-01", "session-a"},
		"b": {"io-b", "/grants/b", repoB, "00-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-bbbbbbbbbbbbbbbb-00", "session-b"},
	}
	type outcome struct {
		client  string
		payload map[string]any
		err     error
	}
	results := make(chan outcome, 2)
	for name, client := range clients {
		go func() {
			payload, isError, rpcErr, err := server.callTool(ctx, revisionNew, "issueops_execution", map[string]any{"action": "status", "id": client.id, "authority_file": client.grant}, map[string]string{"traceparent": client.trace})
			if err == nil && (isError || rpcErr != nil) {
				err = io.ErrUnexpectedEOF
			}
			results <- outcome{client: name, payload: payload, err: err}
		}()
	}
	seen := map[string]entry{}
	for range 2 {
		select {
		case got := <-entered:
			seen[got.id] = got
		case <-ctx.Done():
			t.Fatal("both requests did not reach the handler concurrently")
		}
	}
	close(release)
	for range 2 {
		select {
		case got := <-results:
			client := clients[got.client]
			if got.err != nil || got.payload["id"] != client.id || got.payload["session"] != client.session {
				t.Fatalf("client %s result=%v err=%v", got.client, got.payload, got.err)
			}
		case <-ctx.Done():
			t.Fatal("responses did not arrive")
		}
	}
	for _, client := range clients {
		got := seen[client.id]
		if got.session != client.session || got.cwd != client.root || got.grant != client.grant || got.trace != client.trace {
			t.Fatalf("request %s crossed scope: %+v", client.id, got)
		}
	}
}

func TestHTTPInvalidTraceparentWarnsWithoutEchoingIt(t *testing.T) {
	deps := testTransportServices()
	deps.BindTrace = func(ctx context.Context, raw string) (context.Context, bool) { return ctx, raw == "" }
	server := startHTTPTestServer(t, deps, nil)
	const private = "private-trace-header-value"
	if _, _, _, err := server.callTool(t.Context(), revisionInitialize, "contract_schema", map[string]any{}, map[string]string{"traceparent": private}); err != nil {
		t.Fatal(err)
	}
	log := server.access.String()
	if !strings.Contains(log, "invalid_traceparent_ignored") || strings.Contains(log, private) {
		t.Fatalf("access log = %s", log)
	}
}

func TestHTTPDisconnectCancellationFollowsNegotiatedRevision(t *testing.T) {
	for _, tc := range []struct {
		revision  string
		cancelled bool
	}{{revisionNew, true}, {revisionInitialize, false}} {
		t.Run(tc.revision, func(t *testing.T) {
			repo := canonicalTempDir(t)
			deps := testTransportServices()
			entered, release, handlerErr := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			deps.Execution.ExecuteExecution = func(ctx context.Context, _ string, _ issueopscontract.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
				close(entered)
				select {
				case <-ctx.Done():
				case <-release:
				}
				handlerErr <- ctx.Err()
				return map[string]any{"ok": true}, nil
			}
			disconnected := make(chan struct{})
			deps = withFakeAuthority(deps, map[string]fakeGrant{"/grants/a": {root: repo, identity: fakeIdentity("session-a")}}, map[string][]string{"io-a": {repo}})
			server := startHTTPTestServer(t, deps, func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					go func() {
						<-r.Context().Done()
						close(disconnected)
					}()
					next.ServeHTTP(w, r)
				})
			})
			bound, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			client, disconnect := context.WithCancel(bound)
			go func() {
				_, _, _, _ = server.callTool(client, tc.revision, "issueops_execution", map[string]any{"action": "release", "id": "io-a", "authority_file": "/grants/a"}, nil)
			}()
			select {
			case <-entered:
			case <-bound.Done():
				t.Fatal("handler did not start")
			}
			disconnect()
			select {
			case <-disconnected:
			case <-bound.Done():
				t.Fatal("server did not observe the client disconnect")
			}
			if !tc.cancelled {
				close(release)
			}
			select {
			case err := <-handlerErr:
				if (err != nil) != tc.cancelled {
					t.Fatalf("handler ctx err = %v, want cancelled=%v", err, tc.cancelled)
				}
			case <-bound.Done():
				t.Fatal("handler did not finish")
			}
		})
	}
}

func TestChannelRecvWaitStopsWhenHTTPRequestIsCancelled(t *testing.T) {
	deps := testTransportServices()
	entered, stopped := make(chan struct{}), make(chan error, 1)
	var enter sync.Once
	deps.Channel = channelapp.Service{Effects: channel.Store{
		Root: filepath.Join(t.TempDir(), "channel"), Clock: time.Now,
		OpenDatabase:      func(dir string) (channel.StateDatabase, error) { return sqlstore.Open(dir) },
		WalkExistingAfter: sqlstore.WalkExistingAfter,
		Sleep: func(ctx context.Context, duration time.Duration) error {
			enter.Do(func() { close(entered) })
			err := issueopsadapter.SleepWithContext(ctx, duration)
			if err != nil {
				stopped <- err
			}
			return err
		},
	}}
	server := startHTTPTestServer(t, deps, nil)
	bound, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, disconnect := context.WithCancel(bound)
	go func() {
		_, _, _, _ = server.callTool(client, revisionNew, "channel_recv", map[string]any{"channel": "c", "wait": true, "timeout_seconds": 300}, nil)
	}()
	select {
	case <-entered:
	case <-bound.Done():
		t.Fatal("recv did not start waiting")
	}
	disconnect()
	select {
	case err := <-stopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("wait error = %v", err)
		}
	case <-bound.Done():
		t.Fatal("recv kept waiting after the HTTP request was cancelled")
	}
}

func TestStdioCapabilityBindsCallerWithoutServerAncestry(t *testing.T) {
	repo := canonicalTempDir(t)
	identity := fakeIdentity("session-a")
	deps := testTransportServices()
	var observed atomic.Int32
	deps.Execution.ObserveNativeProcessAncestry = func(int) ([]issueopscontract.NativeProcessReceipt, error) {
		observed.Add(1)
		return nil, nil
	}
	var got issueopscontract.ExecutionActionRequest
	deps.Execution.ExecuteExecution = func(_ context.Context, _ string, request issueopscontract.ExecutionActionRequest, _ port.ExecutionActionDependencies) (any, error) {
		got = request
		return map[string]any{"ok": true}, nil
	}
	deps = withFakeAuthority(deps, map[string]fakeGrant{"/grants/a": {root: repo, identity: identity}}, map[string][]string{"io-a": {repo}})
	call := func(args map[string]any) {
		t.Helper()
		if outcome := dispatchMCPToolCall(t.Context(), MCPToolCall{Name: "issueops_execution", Arguments: args}, deps, transportStdio); outcome.IsError || outcome.Err != nil {
			t.Fatalf("outcome = %+v", outcome)
		}
	}
	call(map[string]any{"action": "status", "id": "io-a", "authority_file": "/grants/a"})
	if got.Actor.SessionID != "session-a" || got.Actor.ProcessAncestry != nil || observed.Load() != 0 {
		t.Fatalf("capability actor = %+v observed=%d", got.Actor, observed.Load())
	}
	call(map[string]any{"action": "status", "id": "io-a", "authority_file": "/grants/a", "host": "codex", "session_id": "session-a"})
	if got.Actor.SessionID != "session-a" || got.Actor.SessionProcess != nil || got.Actor.ProcessAncestry != nil {
		t.Fatalf("stdio supplied actor must reach core Verify unchanged: %+v", got.Actor)
	}
	call(map[string]any{"action": "status", "id": "io-a"})
	if observed.Load() != 1 {
		t.Fatalf("stdio without capability must keep native ancestry, observed=%d", observed.Load())
	}
}
