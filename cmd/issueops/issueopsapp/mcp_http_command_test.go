package issueopsapp

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"issueops/cmd/issueops/mcpcli"
	issueopscore "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	authorityapp "issueops/internal/application/authority"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// startProductionHTTPServer runs the real foreground entry point on an
// ephemeral loopback port and waits for its ready line instead of polling.
func startProductionHTTPServer(t *testing.T, deps mcpcli.MCPDependencies) (string, *lockedBuffer) {
	t.Helper()
	reader, writer := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serveMCPHTTP(ctx, "127.0.0.1:0", statestore.StateDir(), deps, writer)
		_ = writer.Close()
	}()
	lines := bufio.NewReader(reader)
	ready, err := lines.ReadString('\n')
	if err != nil {
		cancel()
		t.Fatalf("server did not report ready: %v (%v)", err, <-done)
	}
	url := ""
	for _, field := range strings.Fields(ready) {
		if value, ok := strings.CutPrefix(field, "url="); ok {
			url = value
		}
	}
	if url == "" {
		t.Fatalf("ready line without url: %q", ready)
	}
	logs := &lockedBuffer{}
	copied := make(chan struct{})
	go func() {
		_, _ = io.Copy(logs, lines)
		close(copied)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("server did not drain")
		}
		<-copied
	})
	return url, logs
}

type headerTransport struct {
	header http.Header
}

func (h headerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	for key, values := range h.header {
		clone.Header[key] = values
	}
	return http.DefaultTransport.RoundTrip(clone)
}

func connectHTTPClient(t *testing.T, url, bearer, traceparent string) *mcp.ClientSession {
	t.Helper()
	header := http.Header{"Authorization": {"Bearer " + bearer}}
	if traceparent != "" {
		header.Set("traceparent", traceparent)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "http-e2e", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint: url, HTTPClient: &http.Client{Transport: headerTransport{header: header}}, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func callHTTPTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) (map[string]any, bool) {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s protocol error: %v", name, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if item, ok := content.(*mcp.TextContent); ok {
			text.WriteString(item.Text)
		}
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text.String()), &payload); err != nil {
		t.Fatalf("%s result is not JSON: %v %q", name, err, text.String())
	}
	return payload, result.IsError
}

func gitRepoForHTTPTest(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", dir, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, output)
	}
	return dir
}

func authorizeSessionForTest(t *testing.T, workspace, session string, receipt model.NativeProcessReceipt) string {
	t.Helper()
	var stdout bytes.Buffer
	err := mcpAuthorizeCommand{issuer: newAuthorityService(), observe: issueopscore.ObserveNativeProcessAncestry, pid: os.Getpid, stdout: &stdout}.Run([]string{
		"--workspace-root", workspace, "--host", "codex", "--session-id", session,
		"--session-pid", strconv.Itoa(receipt.PID), "--session-started-at", receipt.StartedAt, "--session-executable", receipt.Executable, "--json",
	})
	var printed authoritycontract.Receipt
	if decodeErr := json.Unmarshal(stdout.Bytes(), &printed); err != nil || decodeErr != nil || !printed.OK {
		t.Fatalf("authorize %s: %v %v %q", session, err, decodeErr, stdout.String())
	}
	return printed.AuthorityFile
}

func gitWorktreeForHTTPTest(t *testing.T, repo, branch string) string {
	t.Helper()
	worktree := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-"+branch)
	for _, args := range [][]string{
		{"-c", "user.name=HTTP Test", "-c", "user.email=http@example.invalid", "commit", "-q", "--allow-empty", "-m", "test: seed"},
		{"worktree", "add", "-q", "-b", branch, worktree},
	} {
		if output, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	return worktree
}

func seedHTTPLeaseRecord(t *testing.T, repo, worktree string, holder model.NativeActor) model.IssueOpsRecord {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	record := model.IssueOpsRecord{
		OK: true, SchemaVersion: model.IssueOpsCurrentSchemaVersion,
		ID: (issueopscore.CycleStartIdentity{}).StableID(repo, "300-http-lease"), Repo: repo, Branch: "300-http-lease",
		Phase: model.IssueOpsPhaseImplement, WorktreePath: worktree,
		Execution: &model.Execution{
			Mode:      model.ExecutionModeDirect,
			Workspace: model.Workspace{SourceRoot: repo, Root: worktree, Branch: "300-http-lease", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: now},
			Lease:     model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder, ClaimedAt: now},
		},
		CreatedAt: now, UpdatedAt: now,
	}
	written, err := (issueopscore.CycleRecordStore{StateRoot: issueOpsStateRoot()}).Save(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

func requireToolCode(t *testing.T, payload map[string]any, isError bool, code string) {
	t.Helper()
	if !isError || payload["error_code"] != code {
		t.Fatalf("payload=%v isError=%v, want %s", payload, isError, code)
	}
}

func TestMCPHTTPServesTwoClientsWithRequestScopedAuthority(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repoA, repoB := gitRepoForHTTPTest(t), gitRepoForHTTPTest(t)
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grantA := authorizeSessionForTest(t, repoA, "client-a", self)
	grantB := authorizeSessionForTest(t, repoB, "client-b", self)
	grantBOnA := authorizeSessionForTest(t, repoA, "client-b", self)
	holder := model.NativeActor{Host: "codex", SessionID: "client-a", SessionProcess: &self}
	worktreeA := gitWorktreeForHTTPTest(t, repoA, "300-http-lease")
	record := seedHTTPLeaseRecord(t, repoA, worktreeA, holder)

	url, logs := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	clientA := connectHTTPClient(t, url, bearer, "00-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-aaaaaaaaaaaaaaaa-01")
	clientB := connectHTTPClient(t, url, bearer, "00-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-bbbbbbbbbbbbbbbb-01")

	var wait sync.WaitGroup
	routes := make([]map[string]any, 2)
	for index, call := range []struct {
		session *mcp.ClientSession
		grant   string
		repo    string
	}{{clientA, grantA, repoA}, {clientB, grantB, repoB}} {
		wait.Go(func() {
			payload, isError := callHTTPTool(t, call.session, "atomic_commit_preflight", map[string]any{"authority_file": call.grant, "path": call.repo})
			if isError {
				t.Errorf("client %d preflight payload=%v", index, payload)
			}
			routes[index] = payload
		})
	}
	wait.Wait()
	for index, repo := range []string{repoA, repoB} {
		if encoded, _ := json.Marshal(routes[index]); !strings.Contains(string(encoded), repo) || strings.Contains(string(encoded), []string{repoB, repoA}[index]) {
			t.Fatalf("client %d preflight crossed workspaces: %s", index, encoded)
		}
	}

	payload, isError := callHTTPTool(t, clientA, "atomic_commit_preflight", map[string]any{"path": repoA})
	requireToolCode(t, payload, isError, authoritycontract.CodeRequired)
	payload, isError = callHTTPTool(t, clientA, "atomic_commit_preflight", map[string]any{"authority_file": grantA, "path": repoB})
	requireToolCode(t, payload, isError, authoritycontract.CodeInvalid)
	payload, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "status", "id": record.ID, "authority_file": grantA, "session_id": "client-a"})
	requireToolCode(t, payload, isError, authoritycontract.CodeInvalid)
	payload, isError = callHTTPTool(t, clientB, "issueops_execution", map[string]any{"action": "status", "id": record.ID, "authority_file": grantB})
	requireToolCode(t, payload, isError, authoritycontract.CodeInvalid)

	payload, isError = callHTTPTool(t, clientB, "issueops_execution", map[string]any{"action": "release", "id": record.ID, "generation": 1, "authority_file": grantBOnA, "cwd": worktreeA})
	if !isError || !strings.Contains(payload["error"].(string), "holder") {
		t.Fatalf("other holder release payload=%v isError=%v", payload, isError)
	}
	payload, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "release", "id": record.ID, "generation": 2, "authority_file": grantA, "cwd": worktreeA})
	if !isError {
		t.Fatalf("stale generation release succeeded: %v", payload)
	}
	payload, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "release", "id": record.ID, "generation": 1, "authority_file": grantA, "cwd": worktreeA})
	if isError {
		t.Fatalf("holder release payload=%v", payload)
	}
	stored, err := issueopscore.ReadIssueOps(issueOpsStateRoot(), record.ID)
	if err != nil || stored.Execution.Lease.Status == model.LeaseStatusActive || stored.Execution.Lease.Generation != 1 {
		t.Fatalf("lease after HTTP release = %+v err=%v", stored.Execution.Lease, err)
	}
	payload, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "status", "id": record.ID, "authority_file": grantA})
	if isError || payload["id"] != record.ID {
		t.Fatalf("status readback payload=%v", payload)
	}

	rotated := authorizeSessionForTest(t, repoA, "client-a", self)
	payload, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "status", "id": record.ID, "authority_file": grantA})
	requireToolCode(t, payload, isError, authoritycontract.CodeInvalid)
	if _, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "status", "id": record.ID, "authority_file": rotated}); isError {
		t.Fatal("rotated credential was rejected")
	}
	payload, isError = callHTTPTool(t, clientA, "issueops_execution", map[string]any{"action": "status", "id": record.ID, "authority_file": filepath.Join(statestore.StateDir(), "mcp-http", "grants", "missing")})
	requireToolCode(t, payload, isError, authoritycontract.CodeInvalid)

	_, token, err := (authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()}).Read(t.Context(), rotated)
	if err != nil {
		t.Fatal(err)
	}
	log := logs.String()
	for _, secret := range []string{bearer, token, "grants/", repoA} {
		if strings.Contains(log, secret) {
			t.Fatalf("access log leaked %q:\n%s", secret, log)
		}
	}
	if !strings.Contains(log, "tool=issueops_execution") || !strings.Contains(log, "outcome=tool_error") {
		t.Fatalf("access log lacks allowlisted tool events:\n%s", log)
	}
}

func TestMCPHTTPRejectsExpiredCapability(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	repo := gitRepoForHTTPTest(t)
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	grant := authorizeSessionForTest(t, repo, "client-a", self)
	later := authorityapp.New(
		authorityoutbound.Repository{StateRoot: issueOpsStateRoot()}, issueopscore.NativeProcessInspector{},
		authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()},
		func() time.Time { return time.Now().Add(13 * time.Hour) }, authorityoutbound.ScopeResolver{}, rand.Reader,
	)
	deps := issueOpsMCPHTTPDependencies()
	deps.BindAuthority = func(ctx context.Context, use authoritycontract.Use) (context.Context, model.VerifiedActor, error) {
		bound, verified, err := later.Bind(ctx, use)
		if err != nil {
			return ctx, model.VerifiedActor{}, err
		}
		return sqlstore.WithRecordGuard(bound, issueOpsStateRoot(), later.BindSpan), verified, nil
	}
	url, _ := startProductionHTTPServer(t, deps)
	bearer, err := mcpcli.EnsureHTTPBearer(statestore.StateDir())
	if err != nil {
		t.Fatal(err)
	}
	payload, isError := callHTTPTool(t, connectHTTPClient(t, url, bearer, ""), "atomic_commit_preflight", map[string]any{"authority_file": grant, "path": repo})
	requireToolCode(t, payload, isError, authoritycontract.CodeInvalid)
	if !strings.Contains(payload["error"].(string), "expired") {
		t.Fatalf("expired capability payload=%v", payload)
	}
}

func TestMCPHTTPForegroundRejectsNonLoopbackAndBusyAddress(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	if err := serveMCPHTTP(t.Context(), "0.0.0.0:0", statestore.StateDir(), issueOpsMCPHTTPDependencies(), io.Discard); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback err = %v", err)
	}
	url, _ := startProductionHTTPServer(t, issueOpsMCPHTTPDependencies())
	address := strings.TrimSuffix(strings.TrimPrefix(url, "http://"), mcpcli.HTTPEndpointPath)
	if err := serveMCPHTTP(t.Context(), address, statestore.StateDir(), issueOpsMCPHTTPDependencies(), io.Discard); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("busy address err = %v", err)
	}
}
