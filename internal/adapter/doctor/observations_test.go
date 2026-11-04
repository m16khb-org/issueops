package doctor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/projectdoc"
	"issueops/internal/testsupport"
)

func writeFileForTest(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestObserveProjectDocsListsOnlyMissingRequiredDocs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, projectdoc.ProjectDocsDir)
	names := projectdoc.Names()
	for _, name := range names[1:] {
		writeFileForTest(t, filepath.Join(dir, name), "x")
	}
	got := ObserveProjectDocs(root)
	if got.Directory != dir || !slices.Equal(got.Missing, names[:1]) {
		t.Fatalf("observation = %+v, want only %q missing", got, names[0])
	}
	if all := ObserveProjectDocs(t.TempDir()); len(all.Missing) != len(names) {
		t.Fatalf("empty repo missing %d docs, want %d", len(all.Missing), len(names))
	}
}

func TestObserveRuntimeStateReportsOnlyExistingPathsAndDocument(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, projectdoc.ProjectDocsDir)
	empty := ObserveRuntimeState(root)
	if empty.DocumentPath != filepath.Join(dir, "STATE.md") || len(empty.Paths) != 0 || empty.Document != "" {
		t.Fatalf("empty repo observation = %+v", empty)
	}
	if err := os.MkdirAll(filepath.Join(dir, "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFileForTest(t, filepath.Join(dir, "STATE.md"), "# runtime\n")
	got := ObserveRuntimeState(root)
	if !slices.Equal(got.Paths, []string{filepath.Join(dir, "state")}) || got.Document != "# runtime\n" {
		t.Fatalf("observation = %+v", got)
	}
	writeFileForTest(t, filepath.Join(dir, "state.schema.json"), "{}")
	got = ObserveRuntimeState(root)
	if !slices.Equal(got.Paths, []string{filepath.Join(dir, "state"), filepath.Join(dir, "state.schema.json")}) {
		t.Fatalf("paths = %v, want state dir then schema in that order", got.Paths)
	}
}

func TestObserveNativeIntegrationsFlagsMissingCodexHooks(t *testing.T) {
	if got := ObserveNativeIntegrations(""); got.HooksPath != "" || got.HooksMissing || got.Home != "" {
		t.Fatalf("empty home observation = %+v", got)
	}
	home := t.TempDir()
	hooks := filepath.Join(home, ".codex", "hooks.json")
	if got := ObserveNativeIntegrations(home); got.HooksPath != hooks || !got.HooksMissing {
		t.Fatalf("missing hooks observation = %+v", got)
	}
	writeFileForTest(t, hooks, "{}")
	if got := ObserveNativeIntegrations(home); got.HooksPath != hooks || got.HooksMissing {
		t.Fatalf("present hooks observation = %+v", got)
	}
}

func TestObserveBinaryDriftTracksNewestGoSourceAgainstBinary(t *testing.T) {
	if got := ObserveBinaryDrift(""); got.Found || got.Path != "" {
		t.Fatalf("empty root observation = %+v", got)
	}
	root := t.TempDir()
	if got := ObserveBinaryDrift(root); got.Found || got.Path != filepath.Join(root, "bin", testsupport.ExecutableName("issueops")) {
		t.Fatalf("no binary observation = %+v", got)
	}
	built := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	setTime := func(path string, at time.Time) {
		t.Helper()
		if err := os.Chtimes(path, at, at); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(root, "bin", testsupport.ExecutableName("issueops"))
	writeFileForTest(t, binary, "bin")
	setTime(binary, built)
	older := filepath.Join(root, "cmd", "issueops", "main.go")
	writeFileForTest(t, older, "package main")
	setTime(older, built.Add(-time.Hour))
	got := ObserveBinaryDrift(root)
	if !got.Found || !got.BuiltAt.Equal(built) || !got.LatestSource.Equal(built) {
		t.Fatalf("older sources must not move LatestSource: %+v", got)
	}
	newest := filepath.Join(root, "internal", "x", "x.go")
	writeFileForTest(t, newest, "package x")
	setTime(newest, built.Add(2*time.Hour))
	// Only production .go files count: a newer non-Go file, a newer .go file
	// outside cmd/ and internal/, a newer _test.go, and newer testdata Go are
	// not compiled into the binary and are ignored.
	notGo := filepath.Join(root, "internal", "x", "notes.txt")
	writeFileForTest(t, notGo, "n")
	setTime(notGo, built.Add(9*time.Hour))
	outside := filepath.Join(root, "scripts", "tool.go")
	writeFileForTest(t, outside, "package scripts")
	setTime(outside, built.Add(9*time.Hour))
	testFile := filepath.Join(root, "internal", "x", "x_test.go")
	writeFileForTest(t, testFile, "package x")
	setTime(testFile, built.Add(9*time.Hour))
	fixture := filepath.Join(root, "internal", "x", "testdata", "fixture.go")
	writeFileForTest(t, fixture, "package fixture")
	setTime(fixture, built.Add(9*time.Hour))
	got = ObserveBinaryDrift(root)
	if !got.LatestSource.Equal(built.Add(2 * time.Hour)) {
		t.Fatalf("LatestSource = %v, want the newest .go under cmd/ and internal/", got.LatestSource)
	}
}

func TestGatewaysObserveProbesLoopbackEndpointsAndCountsFDsPerUniquePort(t *testing.T) {
	home := t.TempDir()
	if got := (Gateways{}).Observe(t.Context(), ""); got.Home != "" || got.ConfigError != nil || len(got.Endpoints) != 0 {
		t.Fatalf("empty home observation = %+v", got)
	}
	if got := (Gateways{}).Observe(t.Context(), home); got.ConfigError != nil || len(got.Endpoints) != 0 || len(got.FDs) != 0 {
		t.Fatalf("home without config = %+v", got)
	}
	writeFileForTest(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{
		"b":{"type":"http","url":"http://127.0.0.1:7001/mcp"},
		"a":{"type":"sse","url":"http://localhost:7001/sse"},
		"c":{"type":"http","url":"http://[::1]:7002/mcp"},
		"remote":{"type":"http","url":"https://example.com/mcp"},
		"stdio":{"type":"stdio","url":"http://127.0.0.1:7003"}}}`)
	probeErr := errors.New("refused")
	var mu sync.Mutex
	var probed []string
	var counted []int
	gateways := Gateways{
		Probe: func(_ context.Context, target string) error {
			mu.Lock()
			probed = append(probed, target)
			mu.Unlock()
			if strings.Contains(target, "7002") {
				return probeErr
			}
			return nil
		},
		CountFDs: func(_ context.Context, ports []int) map[int]GatewayFDResult {
			mu.Lock()
			counted = append(counted, ports...)
			mu.Unlock()
			return map[int]GatewayFDResult{7001: {Count: 12}, 7002: {Err: errors.New("no lsof")}}
		},
	}
	got := gateways.Observe(t.Context(), home)
	if len(got.Endpoints) != 3 || got.Endpoints[0].Name != "a" || got.Endpoints[1].Name != "b" || got.Endpoints[2].Name != "c" {
		t.Fatalf("endpoints = %+v, want loopback a,b,c in name order", got.Endpoints)
	}
	if got.Endpoints[0].Error != nil || got.Endpoints[1].Error != nil || !errors.Is(got.Endpoints[2].Error, probeErr) {
		t.Fatalf("probe errors not attributed per endpoint: %+v", got.Endpoints)
	}
	slices.Sort(probed)
	if !slices.Equal(probed, []string{"http://127.0.0.1:7001/mcp", "http://[::1]:7002/mcp", "http://localhost:7001/sse"}) {
		t.Fatalf("probed targets = %v", probed)
	}
	if !slices.Equal(counted, []int{7001, 7002}) || len(got.FDs) != 2 {
		t.Fatalf("counted ports = %v fds = %+v", counted, got.FDs)
	}
	if got.FDs[0].Port != 7001 || got.FDs[0].Count != 12 || !got.FDs[0].Available || got.FDs[1].Port != 7002 || got.FDs[1].Available {
		t.Fatalf("fds = %+v", got.FDs)
	}
}

func TestGatewaysObserveProbesEndpointsConcurrentlyInEndpointOrder(t *testing.T) {
	home := t.TempDir()
	writeFileForTest(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{
		"a":{"type":"http","url":"http://127.0.0.1:7001/mcp"},
		"b":{"type":"http","url":"http://127.0.0.1:7002/mcp"},
		"c":{"type":"http","url":"http://127.0.0.1:7003/mcp"}}}`)
	// Each probe waits for all three to be in flight, so a sequential
	// implementation can never get past the first one.
	var inFlight sync.WaitGroup
	inFlight.Add(3)
	gateways := Gateways{
		Probe: func(_ context.Context, target string) error {
			inFlight.Done()
			done := make(chan struct{})
			go func() { inFlight.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				return errors.New("probes ran sequentially")
			}
			if strings.Contains(target, "7002") {
				return errors.New("refused")
			}
			return nil
		},
		CountFDs: func(_ context.Context, ports []int) map[int]GatewayFDResult { return map[int]GatewayFDResult{} },
	}
	got := gateways.Observe(t.Context(), home)
	if len(got.Endpoints) != 3 {
		t.Fatalf("endpoints = %+v", got.Endpoints)
	}
	for i, want := range []string{"a", "b", "c"} {
		if got.Endpoints[i].Name != want {
			t.Fatalf("endpoint %d = %q, want %q (name order)", i, got.Endpoints[i].Name, want)
		}
	}
	if got.Endpoints[0].Error != nil || got.Endpoints[1].Error == nil || got.Endpoints[1].Error.Error() != "refused" || got.Endpoints[2].Error != nil {
		t.Fatalf("probe errors = %+v", got.Endpoints)
	}
}

func TestGatewaysObserveCountsFDsOnceForAllPorts(t *testing.T) {
	home := t.TempDir()
	writeFileForTest(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{
		"a":{"type":"http","url":"http://127.0.0.1:7002/mcp"},
		"b":{"type":"http","url":"http://127.0.0.1:7001/mcp"},
		"c":{"type":"http","url":"http://127.0.0.1:7003/mcp"}}}`)
	var batches [][]int
	gateways := Gateways{
		Probe: func(context.Context, string) error { return nil },
		CountFDs: func(_ context.Context, ports []int) map[int]GatewayFDResult {
			batches = append(batches, slices.Clone(ports))
			return map[int]GatewayFDResult{7001: {Count: 5}, 7002: {Err: errors.New("no listener")}}
		},
	}
	got := gateways.Observe(t.Context(), home)
	if len(batches) != 1 || !slices.Equal(batches[0], []int{7001, 7002, 7003}) {
		t.Fatalf("batch calls = %v, want one call with every unique port in order", batches)
	}
	want := []doctordomain.GatewayFD{{Port: 7001, Count: 5, Available: true}, {Port: 7002}, {Port: 7003}}
	if !slices.Equal(got.FDs, want) {
		t.Fatalf("fds = %+v, want %+v (a port the batch omits is unavailable)", got.FDs, want)
	}
}

func TestGatewaysObserveReportsUnparsableConfigWithoutProbing(t *testing.T) {
	home := t.TempDir()
	writeFileForTest(t, filepath.Join(home, ".claude.json"), "{not json")
	gateways := Gateways{
		Probe: func(context.Context, string) error { t.Error("probed despite a broken config"); return nil },
		CountFDs: func(context.Context, []int) map[int]GatewayFDResult {
			t.Error("counted despite a broken config")
			return nil
		},
	}
	got := gateways.Observe(t.Context(), home)
	if got.ConfigError == nil || len(got.Endpoints) != 0 {
		t.Fatalf("observation = %+v", got)
	}
}

func TestLoopbackMCPEndpointsSkipsUnusableEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude.json")
	if got, err := loopbackMCPEndpoints(path); err != nil || got != nil {
		t.Fatalf("missing config = %v, %v", got, err)
	}
	writeFileForTest(t, path, `{"mcpServers":{
		"bad-url":{"type":"http","url":"http://127.0.0.1:%zz"},
		"lan":{"type":"http","url":"http://192.168.0.2:9/mcp"},
		"ok":{"type":"http","url":"http://localhost:9/mcp"}}}`)
	got, err := loopbackMCPEndpoints(path)
	if err != nil || len(got) != 1 || got[0].Name != "ok" {
		t.Fatalf("endpoints = %+v, %v", got, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := loopbackMCPEndpoints(path); err == nil {
		t.Fatal("a directory in place of the config did not error")
	}
}

func TestUniqueMCPGatewayPortsDefaultsByScheme(t *testing.T) {
	parse := func(raw string) mcpGatewayEndpoint {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return mcpGatewayEndpoint{URL: parsed}
	}
	got := uniqueMCPGatewayPorts([]mcpGatewayEndpoint{
		parse("https://localhost/mcp"), parse("http://localhost/mcp"), parse("http://localhost:9000/a"), parse("http://127.0.0.1:9000/b"), parse("https://localhost/other"),
	})
	if !slices.Equal(got, []int{80, 443, 9000}) {
		t.Fatalf("ports = %v", got)
	}
}

func TestProbeGatewayHTTPAcceptsAnyHTTPStatusButNotTransportFailure(t *testing.T) {
	var gotMethod, gotAccept, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotAccept = r.Method, r.Header.Get("Accept")
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	if err := ProbeGatewayHTTP(t.Context(), server.URL); err != nil {
		t.Fatalf("an HTTP 401 proves the listener answers: %v", err)
	}
	if gotMethod != http.MethodPost || !strings.Contains(gotAccept, "text/event-stream") || !strings.Contains(gotBody, `"method":"initialize"`) {
		t.Fatalf("probe sent method=%q accept=%q body=%q", gotMethod, gotAccept, gotBody)
	}
	address := server.Listener.Addr().String()
	server.Close()
	if err := ProbeGatewayHTTP(t.Context(), "http://"+address); err == nil {
		t.Fatal("refused connection reported healthy")
	}
	if err := ProbeGatewayHTTP(t.Context(), "http://bad host/"); err == nil {
		t.Fatal("malformed URL reported healthy")
	}
}

func TestMeasurePipeCapacityReportsTheKernelBufferQuickly(t *testing.T) {
	started := time.Now()
	capacity, err := MeasurePipeCapacity()
	if err != nil {
		t.Fatal(err)
	}
	// Every supported kernel buffers at least one page and at most the probe cap.
	if capacity < 4096 || capacity > 1<<20 {
		t.Fatalf("capacity = %d, want a real pipe buffer size", capacity)
	}
	if elapsed := time.Since(started); elapsed > 50*time.Millisecond {
		t.Fatalf("probe took %s; it must not wait on an idle timer", elapsed)
	}
}
