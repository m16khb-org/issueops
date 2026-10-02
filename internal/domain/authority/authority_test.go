package authority

import (
	"errors"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/authority"
)

var issuedAt = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

func testActor() contract.NativeActor {
	return contract.NativeActor{
		Host: " CODEX ", SessionID: " session ", AgentID: " agent ",
		SessionProcess:  &contract.ProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"},
		ProcessAncestry: []contract.ProcessReceipt{{PID: 42, StartedAt: "start", Executable: "/bin/codex"}},
	}
}

func gitScope() contract.Scope {
	return contract.Scope{WorkspaceRoot: "/repo.worktrees/a", CWD: "/repo.worktrees/a", SourceRoot: "/repo.worktrees/a", GitCommonDir: "/repo/.git"}
}

func testRecord(t *testing.T, token string) contract.Record {
	t.Helper()
	key, ok := TokenKey(token)
	if !ok {
		t.Fatalf("token key")
	}
	return NewRecord(key, gitScope(), testActor(), TokenDigest(token), issuedAt)
}

func testToken(scope contract.Scope) string {
	return ComposeToken(Key(scope, testActor()), "c2VjcmV0")
}

func TestKeyBindsScopeAnchorAndNormalizedIdentity(t *testing.T) {
	base := Key(gitScope(), testActor())
	worktree := gitScope()
	worktree.WorkspaceRoot, worktree.SourceRoot = "/repo", "/repo"
	if Key(worktree, testActor()) != base {
		t.Fatal("worktrees of one repository must share the grant key")
	}
	other := gitScope()
	other.GitCommonDir = "/other/.git"
	agent := testActor()
	agent.AgentID = "other"
	directory := contract.Scope{SourceRoot: "/repo/.git"}
	for name, key := range map[string]string{
		"repository": Key(other, testActor()),
		"agent":      Key(gitScope(), agent),
		"scope kind": Key(directory, testActor()),
	} {
		if key == base {
			t.Fatalf("%s change kept the same key", name)
		}
	}
	if !ValidKey(base) {
		t.Fatalf("key %q is not 64 lowercase hex", base)
	}
}

func TestRecordRoundTripStripsAncestryAndRejectsUnsupportedSchema(t *testing.T) {
	token := testToken(gitScope())
	record := testRecord(t, token)
	record.Actor.ProcessAncestry = testActor().ProcessAncestry
	data, err := EncodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "ancestry") || strings.Contains(string(data), "c2VjcmV0") {
		t.Fatalf("stored grant leaked ancestry or secret: %s", data)
	}
	decoded, err := DecodeRecord(data, record.Key)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Actor.ProcessAncestry != nil || decoded.Actor.Host != "codex" || decoded.Actor.SessionID != "session" {
		t.Fatalf("decoded actor=%+v", decoded.Actor)
	}
	if decoded.ExpiresAt != issuedAt.Add(12*time.Hour).Format(time.RFC3339Nano) {
		t.Fatalf("expires_at=%s", decoded.ExpiresAt)
	}
	valid := string(data)
	for name, raw := range map[string]string{
		"malformed":     "{",
		"missing":       strings.Replace(valid, `"schema_version":1,`, "", 1),
		"zero":          strings.Replace(valid, `"schema_version":1`, `"schema_version":0`, 1),
		"future":        strings.Replace(valid, `"schema_version":1`, `"schema_version":2`, 1),
		"unknown field": strings.Replace(valid, `{"schema_version":1`, `{"verified":true,"schema_version":1`, 1),
		"trailing":      valid + "{}",
	} {
		if _, err := DecodeRecord([]byte(raw), record.Key); !errors.Is(err, contract.ErrInvalidState) {
			t.Fatalf("%s grant err=%v", name, err)
		}
	}
	if _, err := DecodeRecord(data, strings.Repeat("0", 64)); !errors.Is(err, contract.ErrInvalidState) {
		t.Fatalf("foreign key err=%v", err)
	}
}

func TestCheckRejectsRevokedExpiredAndForeignScope(t *testing.T) {
	token := testToken(gitScope())
	record := testRecord(t, token)
	if err := Check(record, token, gitScope(), issuedAt.Add(TTL-time.Nanosecond)); err != nil {
		t.Fatalf("valid grant: %v", err)
	}
	foreign := gitScope()
	foreign.GitCommonDir = "/other/.git"
	for name, check := range map[string]func() error{
		"revoked":   func() error { return Check(record, ComposeToken(record.Key, "b2xk"), gitScope(), issuedAt) },
		"malformed": func() error { return Check(record, "not-a-token", gitScope(), issuedAt) },
		"expired":   func() error { return Check(record, token, gitScope(), issuedAt.Add(TTL)) },
		"repo":      func() error { return Check(record, token, foreign, issuedAt) },
		"non-git":   func() error { return Check(record, token, contract.Scope{SourceRoot: "/repo"}, issuedAt) },
	} {
		if authorityErr, ok := errors.AsType[*Error](check()); !ok || authorityErr.Code != contract.CodeInvalid {
			t.Fatalf("%s err=%v", name, check())
		}
	}
}

func TestDirectoryScopeIsBoundedBySourceRoot(t *testing.T) {
	granted := contract.Scope{WorkspaceRoot: "/work", CWD: "/work", SourceRoot: "/work"}
	token := ComposeToken(Key(granted, testActor()), "c2VjcmV0")
	key, _ := TokenKey(token)
	record := NewRecord(key, granted, testActor(), TokenDigest(token), issuedAt)
	inside := contract.Scope{WorkspaceRoot: "/work/sub", CWD: "/work/sub/dir", SourceRoot: "/work/sub"}
	if err := Check(record, token, inside, issuedAt); err != nil {
		t.Fatalf("nested directory refused: %v", err)
	}
	for _, outside := range []contract.Scope{
		{WorkspaceRoot: "/workspace", CWD: "/workspace", SourceRoot: "/workspace"},
		{WorkspaceRoot: "/", CWD: "/", SourceRoot: "/"},
		{WorkspaceRoot: "/work", CWD: "/work", SourceRoot: "/work", GitCommonDir: "/work/.git"},
	} {
		if err := Check(record, token, outside, issuedAt); err == nil {
			t.Fatalf("scope %+v escaped the bounded root", outside)
		}
	}
}

func TestMatchIdentityAcceptsEmptyOrSameCaller(t *testing.T) {
	granted := NormalizeIdentity(testActor())
	if err := MatchIdentity(granted, contract.NativeActor{}); err != nil {
		t.Fatal(err)
	}
	if err := MatchIdentity(granted, testActor()); err != nil {
		t.Fatal(err)
	}
	other := testActor()
	other.SessionID = "other"
	reused := testActor()
	reused.SessionProcess = &contract.ProcessReceipt{PID: 42, StartedAt: "later", Executable: "/bin/codex"}
	for _, supplied := range []contract.NativeActor{other, reused} {
		if err := MatchIdentity(granted, supplied); err == nil {
			t.Fatalf("mismatched caller %+v accepted", supplied)
		}
	}
}
