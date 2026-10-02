package authority

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	contract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/authority"
	authorityport "issueops/internal/port/authority"
)

type memoryGrants struct {
	mu      sync.Mutex
	records map[string][]byte
	reads   int
}

func (m *memoryGrants) Within(_ context.Context, key string, fn func(*contract.Record) (*contract.Record, error)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var current *contract.Record
	if data, ok := m.records[key]; ok {
		record, err := domain.DecodeRecord(data, key)
		if err != nil {
			return err
		}
		current = &record
	}
	next, err := fn(current)
	if err != nil || next == nil {
		return err
	}
	data, err := domain.EncodeRecord(*next)
	if err != nil {
		return err
	}
	m.records[key] = data
	return nil
}

func (m *memoryGrants) Get(bucket, id string) ([]byte, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads++
	if bucket != contract.Bucket {
		return nil, false, fmt.Errorf("unexpected bucket %q", bucket)
	}
	data, ok := m.records[id]
	return append([]byte(nil), data...), ok, nil
}

type memoryFiles struct {
	mu    sync.Mutex
	files map[string]string
}

func (f *memoryFiles) Write(_ context.Context, key, token string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	path := "/state/mcp-http/grants/" + key + "/" + domain.TokenDigest(token)
	if _, exists := f.files[path]; exists {
		return "", errors.New("credential file exists")
	}
	f.files[path] = token
	return path, nil
}

func (f *memoryFiles) Read(_ context.Context, path string) (string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.files[path]
	if !ok {
		return "", "", errors.New("missing credential")
	}
	key, _ := domain.TokenKey(token)
	return key, token, nil
}

type scopes map[string]contract.Scope

func (s scopes) Resolve(_ context.Context, root, cwd string) (contract.Scope, error) {
	scope, ok := s[root]
	if !ok {
		return contract.Scope{}, fmt.Errorf("unknown workspace %s", root)
	}
	if cwd != "" {
		scope.CWD = cwd
	}
	return scope, nil
}

type processTable struct {
	mu   sync.Mutex
	live map[int]model.NativeProcessReceipt
}

func (p *processTable) Inspect(ctx context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	if err := ctx.Err(); err != nil {
		return "", model.NativeProcessReceipt{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	observed, ok := p.live[receipt.PID]
	if !ok {
		return "dead", model.NativeProcessReceipt{}, nil
	}
	if observed != receipt {
		return "identity_mismatch", observed, nil
	}
	return "live", observed, nil
}

func (p *processTable) set(receipt model.NativeProcessReceipt) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.live[receipt.PID] = receipt
}

func (p *processTable) kill(pid int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.live, pid)
}

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type counterEntropy struct {
	mu   sync.Mutex
	next byte
}

func (e *counterEntropy) Read(buffer []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for index := range buffer {
		e.next++
		buffer[index] = e.next
	}
	return len(buffer), nil
}

type harness struct {
	grants  *memoryGrants
	files   *memoryFiles
	procs   *processTable
	clock   *clock
	service *Service
}

var (
	repoScope  = contract.Scope{WorkspaceRoot: "/repo", CWD: "/repo", SourceRoot: "/repo", GitCommonDir: "/repo/.git"}
	treeScope  = contract.Scope{WorkspaceRoot: "/repo.worktrees/a", CWD: "/repo.worktrees/a", SourceRoot: "/repo.worktrees/a", GitCommonDir: "/repo/.git"}
	otherScope = contract.Scope{WorkspaceRoot: "/other", CWD: "/other", SourceRoot: "/other", GitCommonDir: "/other/.git"}
)

func newHarness() *harness {
	h := &harness{
		grants: &memoryGrants{records: map[string][]byte{}},
		files:  &memoryFiles{files: map[string]string{}},
		procs:  &processTable{live: map[int]model.NativeProcessReceipt{}},
		clock:  &clock{now: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)},
	}
	h.service = h.restart()
	return h
}

func (h *harness) restart() *Service {
	return New(h.grants, h.procs, h.files, h.clock.Now, scopes{"/repo": repoScope, "/repo.worktrees/a": treeScope, "/other": otherScope}, &counterEntropy{})
}

func (h *harness) nativeActor(session string, pid int) model.NativeActor {
	receipt := model.NativeProcessReceipt{PID: pid, StartedAt: "start-" + session, Executable: "/bin/codex"}
	h.procs.set(receipt)
	return model.NativeActor{Host: "codex", SessionID: session, SessionProcess: &receipt, ProcessAncestry: []model.NativeProcessReceipt{{PID: 1, StartedAt: "init", Executable: "/sbin/init"}, receipt}}
}

func (h *harness) issue(t *testing.T, actor model.NativeActor, root string) contract.Use {
	t.Helper()
	receipt, err := h.service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: root, Actor: actor})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if !receipt.OK || receipt.AuthorityFile == "" {
		t.Fatalf("receipt=%+v", receipt)
	}
	key, token, err := h.files.Read(context.Background(), receipt.AuthorityFile)
	if err != nil {
		t.Fatal(err)
	}
	return contract.Use{Key: key, Token: token, WorkspaceRoot: root}
}

func requireCode(t *testing.T, err error, code string) {
	t.Helper()
	authorityErr, ok := errors.AsType[*domain.Error](err)
	if !ok || authorityErr.Code != code {
		t.Fatalf("err=%v, want %s", err, code)
	}
}

func TestIssueBeforeAnyLeaseStoresOnlyIdentityHashScopeAndExpiry(t *testing.T) {
	h := newHarness()
	actor := h.nativeActor("session", 4242)
	receipt, err := h.service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: "/repo", Actor: actor})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ExpiresAt != "2026-10-02T12:00:00Z" {
		t.Fatalf("expires_at=%s", receipt.ExpiresAt)
	}
	key, token, err := h.files.Read(context.Background(), receipt.AuthorityFile)
	if err != nil {
		t.Fatal(err)
	}
	stored := string(h.grants.records[key])
	if strings.Contains(stored, token) || strings.Contains(stored, strings.SplitN(token, ".", 2)[1]) || strings.Contains(stored, "init") || strings.Contains(stored, receipt.AuthorityFile) {
		t.Fatalf("grant stored a credential, credential path, or ancestry: %s", stored)
	}
	record, err := domain.DecodeRecord(h.grants.records[key], key)
	if err != nil {
		t.Fatal(err)
	}
	if record.Actor.ProcessAncestry != nil || record.GitCommonDir != "/repo/.git" || record.TokenSHA256 != domain.TokenDigest(token) {
		t.Fatalf("record=%+v", record)
	}
}

func TestIssueRequiresObservedAncestryAndLiveSession(t *testing.T) {
	h := newHarness()
	forged := h.nativeActor("session", 4242)
	forged.ProcessAncestry = []model.NativeProcessReceipt{{PID: 1, StartedAt: "init", Executable: "/sbin/init"}}
	if _, err := h.service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: "/repo", Actor: forged}); err == nil || !strings.Contains(err.Error(), "ancestry") {
		t.Fatalf("issue outside ancestry err=%v", err)
	}
	dead := h.nativeActor("dead", 5151)
	h.procs.kill(5151)
	if _, err := h.service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: "/repo", Actor: dead}); err == nil || !strings.Contains(err.Error(), "not live") {
		t.Fatalf("dead session err=%v", err)
	}
	omo := h.nativeActor("omo-session", 6161)
	omo.Host = "omo"
	if _, err := h.service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: "/repo", Actor: omo}); err != nil {
		t.Fatalf("omo native actor refused: %v", err)
	}
	if len(h.grants.records) != 1 {
		t.Fatalf("rejected issues persisted grants: %d", len(h.grants.records))
	}
}

func TestBindVerifiesCapabilityWithNilAncestry(t *testing.T) {
	h := newHarness()
	use := h.issue(t, h.nativeActor("session", 4242), "/repo")
	use.WorkspaceRoot = "/repo.worktrees/a"
	ctx, verified, err := h.service.Bind(context.Background(), use)
	if err != nil {
		t.Fatalf("same-repository worktree bind: %v", err)
	}
	if verified.Method != model.VerifiedByCapability || verified.Identity.ProcessAncestry != nil || verified.Identity.SessionID != "session" {
		t.Fatalf("verified=%+v", verified)
	}
	again, err := h.service.Verify(ctx, model.NativeActor{})
	if err != nil || again.Identity.SessionID != "session" || again.Identity.ProcessAncestry != nil {
		t.Fatalf("verify=%+v err=%v", again, err)
	}
	if _, err := h.service.Verify(ctx, model.NativeActor{Host: "codex", SessionID: "other"}); err == nil {
		t.Fatal("mixed actor input for another session accepted")
	}
	if _, err := h.service.Verify(ctx, model.NativeActor{Host: "codex", SessionID: "session", SessionProcess: verified.Identity.SessionProcess}); err != nil {
		t.Fatalf("matching mixed actor refused: %v", err)
	}
	if _, _, err := h.service.Bind(ctx, use); err == nil {
		t.Fatal("second capability bound onto one request")
	}
	if _, err := h.service.Issue(ctx, contract.IssueRequest{WorkspaceRoot: "/repo", Actor: h.nativeActor("session", 4242)}); err == nil {
		t.Fatal("capability-bound request issued a new grant")
	}
}

func TestBindRejectsMissingMalformedForeignAndWrongCredential(t *testing.T) {
	h := newHarness()
	use := h.issue(t, h.nativeActor("session", 4242), "/repo")
	_, _, err := h.service.Bind(context.Background(), contract.Use{WorkspaceRoot: "/repo"})
	requireCode(t, err, contract.CodeRequired)
	for name, mutate := range map[string]func(*contract.Use){
		"key mismatch":    func(u *contract.Use) { u.Key = strings.Repeat("0", 64) },
		"malformed token": func(u *contract.Use) { u.Token = "garbage" },
		"wrong secret":    func(u *contract.Use) { u.Token = use.Key + ".d3Jvbmc" },
		"foreign repo":    func(u *contract.Use) { u.WorkspaceRoot = "/other" },
		"unknown root":    func(u *contract.Use) { u.WorkspaceRoot = "/elsewhere" },
	} {
		candidate := use
		mutate(&candidate)
		_, _, err := h.service.Bind(context.Background(), candidate)
		requireCode(t, err, contract.CodeInvalid)
		_ = name
	}
}

func TestReissueRevokesPreviousCredential(t *testing.T) {
	h := newHarness()
	actor := h.nativeActor("session", 4242)
	first := h.issue(t, actor, "/repo")
	ctx, _, err := h.service.Bind(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	second := h.issue(t, actor, "/repo")
	if second.Key != first.Key || second.Token == first.Token {
		t.Fatalf("reissue must rotate the same key: first=%s second=%s", first.Key, second.Key)
	}
	_, _, err = h.service.Bind(context.Background(), first)
	requireCode(t, err, contract.CodeInvalid)
	_, err = h.service.Verify(ctx, model.NativeActor{})
	requireCode(t, err, contract.CodeInvalid)
	if _, _, err := h.service.Bind(context.Background(), second); err != nil {
		t.Fatalf("new credential refused: %v", err)
	}
}

func TestCredentialExpiresAtTTL(t *testing.T) {
	h := newHarness()
	use := h.issue(t, h.nativeActor("session", 4242), "/repo")
	ctx, _, err := h.service.Bind(context.Background(), use)
	if err != nil {
		t.Fatal(err)
	}
	h.clock.advance(domain.TTL - time.Nanosecond)
	if _, err := h.service.Verify(ctx, model.NativeActor{}); err != nil {
		t.Fatalf("before expiry: %v", err)
	}
	h.clock.advance(time.Nanosecond)
	_, err = h.service.Verify(ctx, model.NativeActor{})
	requireCode(t, err, contract.CodeInvalid)
	_, _, err = h.service.Bind(context.Background(), use)
	requireCode(t, err, contract.CodeInvalid)
}

func TestServerRestartKeepsGrantButHostRestartAndPIDReuseRevoke(t *testing.T) {
	h := newHarness()
	actor := h.nativeActor("session", 4242)
	use := h.issue(t, actor, "/repo")
	restarted := h.restart()
	if _, _, err := restarted.Bind(context.Background(), use); err != nil {
		t.Fatalf("server restart lost the persisted grant: %v", err)
	}
	h.procs.set(model.NativeProcessReceipt{PID: 4242, StartedAt: "reused", Executable: "/bin/codex"})
	_, _, err := restarted.Bind(context.Background(), use)
	requireCode(t, err, contract.CodeInvalid)
	h.procs.kill(4242)
	_, _, err = restarted.Bind(context.Background(), use)
	requireCode(t, err, contract.CodeInvalid)
}

func TestUnsupportedStoredSchemaFailsClosed(t *testing.T) {
	h := newHarness()
	use := h.issue(t, h.nativeActor("session", 4242), "/repo")
	valid := string(h.grants.records[use.Key])
	for name, raw := range map[string]string{
		"malformed": "{",
		"missing":   strings.Replace(valid, `"schema_version":1,`, "", 1),
		"future":    strings.Replace(valid, `"schema_version":1`, `"schema_version":9`, 1),
	} {
		h.grants.records[use.Key] = []byte(raw)
		if _, _, err := h.service.Bind(context.Background(), use); !errors.Is(err, contract.ErrInvalidState) {
			t.Fatalf("%s grant err=%v", name, err)
		}
		if _, err := h.service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: "/repo", Actor: h.nativeActor("session", 4242)}); !errors.Is(err, contract.ErrInvalidState) {
			t.Fatalf("%s grant was silently overwritten: %v", name, err)
		}
	}
}

func TestUnboundVerifyUsesNativeAncestryAndLiveProcess(t *testing.T) {
	h := newHarness()
	actor := h.nativeActor("session", 4242)
	verified, err := h.service.Verify(context.Background(), actor)
	if err != nil || verified.Method != model.VerifiedByNativeAncestry || len(verified.Identity.ProcessAncestry) != 2 {
		t.Fatalf("native verified=%+v err=%v", verified, err)
	}
	actor.ProcessAncestry = nil
	if _, err := h.service.Verify(context.Background(), actor); err == nil {
		t.Fatal("native caller without observed ancestry accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := h.service.Verify(ctx, h.nativeActor("session", 4242)); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled verify err=%v", err)
	}
}

func TestBindSpanRechecksThroughLockedReaderAndSerializesRotation(t *testing.T) {
	h := newHarness()
	actor := h.nativeActor("session", 4242)
	use := h.issue(t, actor, "/repo")
	unbound := context.Background()
	if got, err := h.service.BindSpan(unbound, nil); err != nil || got != unbound {
		t.Fatalf("unbound span must pass through: ctx=%v err=%v", got, err)
	}
	ctx, _, err := h.service.Bind(context.Background(), use)
	if err != nil {
		t.Fatal(err)
	}
	locked := &memoryGrants{records: map[string][]byte{use.Key: append([]byte(nil), h.grants.records[use.Key]...)}}
	spanCtx, err := h.service.BindSpan(ctx, locked)
	if err != nil {
		t.Fatal(err)
	}
	before := h.grants.reads
	if _, err := h.service.Verify(spanCtx, model.NativeActor{}); err != nil {
		t.Fatal(err)
	}
	if h.grants.reads != before || locked.reads != 2 {
		t.Fatalf("span verify must reuse the locked reader: unspanned=%d locked=%d", h.grants.reads-before, locked.reads)
	}
	h.issue(t, actor, "/repo")
	rotated := &memoryGrants{records: map[string][]byte{use.Key: h.grants.records[use.Key]}}
	_, err = h.service.BindSpan(ctx, rotated)
	requireCode(t, err, contract.CodeInvalid)
	h.clock.advance(domain.TTL)
	_, err = h.service.Verify(spanCtx, model.NativeActor{})
	if err != nil {
		t.Fatalf("span verify must use the span entry instant, not a later clock: %v", err)
	}
}

func TestConcurrentRequestsKeepCapabilitiesIsolated(t *testing.T) {
	h := newHarness()
	alice := h.issue(t, h.nativeActor("alice", 4001), "/repo")
	bob := h.issue(t, h.nativeActor("bob", 4002), "/other")
	start := make(chan struct{})
	errs := make(chan error, 64)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		use, want := alice, "alice"
		if i%2 == 1 {
			use, want = bob, "bob"
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, verified, err := h.service.Bind(context.Background(), use)
			if err != nil {
				errs <- err
				return
			}
			again, err := h.service.Verify(ctx, model.NativeActor{})
			if err != nil || verified.Identity.SessionID != want || again.Identity.SessionID != want {
				errs <- fmt.Errorf("request crossed identities: want %s got %s/%s err=%v", want, verified.Identity.SessionID, again.Identity.SessionID, err)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if _, err := h.service.Verify(context.Background(), model.NativeActor{Host: "codex", SessionID: "alice"}); err == nil {
		t.Fatal("unbound request inherited a capability from another request")
	}
}

var _ authorityport.Repository = (*memoryGrants)(nil)
