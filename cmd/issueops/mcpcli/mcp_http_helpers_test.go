package mcpcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	authoritycontract "issueops/internal/contract/authority"
	issueopscontract "issueops/internal/contract/issueops"
	authoritydomain "issueops/internal/domain/authority"
)

// revisionInitialize는 initialize 핸드셰이크를 쓰는 2025 revision이다. ADR
// 2026-10-02 실측에서 Omo가 2025-11-25, Codex가 2025-06-18로 연결했으므로
// 현행 host 경로다.
const (
	revisionInitialize = "2025-11-25"
	revisionNew        = "2026-07-28"
)

type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

type httpTestServer struct {
	url, address, bearer string
	access               *syncBuffer
}

func startHTTPTestServer(t *testing.T, deps MCPDependencies, wrap func(http.Handler) http.Handler) httpTestServer {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	bearer := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x5a}, 32))
	access := &syncBuffer{}
	handler, err := NewHTTPHandler(deps, HTTPOptions{Address: address, Bearer: bearer, Logger: slog.New(slog.NewTextHandler(access, nil))})
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	if wrap != nil {
		handler = wrap(handler)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ServeHTTP(ctx, listener, handler) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("http server shutdown: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Error("http server did not stop")
		}
	})
	return httpTestServer{url: "http://" + address + HTTPEndpointPath, address: address, bearer: bearer, access: access}
}

func (s httpTestServer) post(ctx context.Context, revision, body string, header map[string]string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("Authorization", "Bearer "+s.bearer)
	if revision != "" {
		request.Header.Set("Mcp-Protocol-Version", revision)
	}
	for key, value := range header {
		switch {
		case key == "Host":
			request.Host = value
		case value == "":
			request.Header.Del(key)
		default:
			request.Header.Set(key, value)
		}
	}
	return http.DefaultClient.Do(request)
}

func toolCallBody(revision, name string, args map[string]any) string {
	params := map[string]any{"name": name, "arguments": args}
	if revision >= revisionNew {
		params["_meta"] = map[string]any{
			"io.modelcontextprotocol/protocolVersion":    revision,
			"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "http-test", "version": "1"},
			"io.modelcontextprotocol/clientCapabilities": map[string]any{},
		}
	}
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 9, "method": "tools/call", "params": params})
	return string(body)
}

func (s httpTestServer) callTool(ctx context.Context, revision, name string, args map[string]any, header map[string]string) (map[string]any, bool, *jsonrpc.Error, error) {
	if revision >= revisionNew {
		merged := map[string]string{"Mcp-Method": "tools/call", "Mcp-Name": name}
		for key, value := range header {
			merged[key] = value
		}
		header = merged
	}
	response, err := s.post(ctx, revision, toolCallBody(revision, name, args), header)
	if err != nil {
		return nil, false, nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(response.Body)
		return nil, false, nil, fmt.Errorf("status %d: %s", response.StatusCode, data)
	}
	message, err := readSSEResponse(response.Body)
	if err != nil {
		return nil, false, nil, err
	}
	if message.Error != nil {
		return nil, false, message.Error, nil
	}
	var result mcp.CallToolResult
	if err := json.Unmarshal(message.Result, &result); err != nil {
		return nil, false, nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(toolResultText(&result)), &payload); err != nil {
		return nil, result.IsError, nil, fmt.Errorf("tool text is not JSON: %w", err)
	}
	return payload, result.IsError, nil, nil
}

func readSSEResponse(body io.Reader) (revisionMessage, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64<<10), 8<<20)
	for scanner.Scan() {
		data, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var message revisionMessage
		if err := json.Unmarshal([]byte(data), &message); err != nil {
			return revisionMessage{}, err
		}
		if message.ID != 0 {
			return message, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return revisionMessage{}, err
	}
	return revisionMessage{}, errors.New("no JSON-RPC response in SSE stream")
}

func canonicalTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

type fakeGrant struct {
	root     string
	identity issueopscontract.NativeActor
}

type boundGrantKey struct{}

type fakeScopeResolver struct{}

func (fakeScopeResolver) Resolve(_ context.Context, root, cwd string) (authoritycontract.Scope, error) {
	if cwd == "" {
		cwd = root
	}
	if !filepath.IsAbs(root) || !pathWithin(cwd, root) {
		return authoritycontract.Scope{}, fmt.Errorf("cwd is outside workspace_root")
	}
	return authoritycontract.Scope{WorkspaceRoot: root, CWD: cwd, SourceRoot: root}, nil
}

type fakeCredentials map[string]fakeGrant

func (f fakeCredentials) Write(context.Context, string, string) (string, error) {
	return "", errors.New("not used")
}

func (f fakeCredentials) Read(_ context.Context, path string) (string, string, error) {
	if _, ok := f[path]; !ok {
		return "", "", errors.New("authority credential path is not a managed credential file")
	}
	return path, "token:" + path, nil
}

// withFakeAuthority wires the real request scope and dispatch pipeline to an
// in-memory grant table that enforces scope like the authority service.
func withFakeAuthority(deps MCPDependencies, grants map[string]fakeGrant, records map[string][]string) MCPDependencies {
	deps.RequestScope = NewRequestScope(fakeScopeResolver{}, func(_ context.Context, _ RecordKind, id string) ([]string, error) {
		roots, ok := records[id]
		if !ok {
			return nil, fmt.Errorf("issueops record %s not found", id)
		}
		return roots, nil
	})
	deps.Credentials = fakeCredentials(grants)
	deps.BindAuthority = func(ctx context.Context, use authoritycontract.Use) (context.Context, issueopscontract.VerifiedActor, error) {
		grant, ok := grants[use.Key]
		if !ok || use.Token != "token:"+use.Key {
			return ctx, issueopscontract.VerifiedActor{}, authoritydomain.Invalid("authority credential was revoked or never issued")
		}
		if !pathWithin(use.WorkspaceRoot, grant.root) || !pathWithin(use.CWD, grant.root) {
			return ctx, issueopscontract.VerifiedActor{}, authoritydomain.Invalid("request workspace is outside the authorized repository scope")
		}
		return context.WithValue(ctx, boundGrantKey{}, use.Key), issueopscontract.VerifiedActor{Identity: grant.identity, Method: issueopscontract.VerifiedByCapability}, nil
	}
	deps.ForRequest = func(ctx context.Context, scope authoritycontract.Scope, _ issueopscontract.VerifiedActor) (context.Context, MCPDependencies, error) {
		request := deps
		request.DefaultTarget = scope.WorkspaceRoot
		return ctx, request, nil
	}
	return deps
}

func fakeIdentity(session string) issueopscontract.NativeActor {
	return issueopscontract.NativeActor{Host: "codex", SessionID: session, SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 4242, StartedAt: "Thu Oct  2 10:00:00 2026", Executable: "/usr/bin/codex"}}
}
