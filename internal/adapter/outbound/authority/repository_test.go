package authority

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	contract "issueops/internal/contract/authority"
	issueopscontract "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/authority"
	"issueops/internal/port"
)

func grantFixture(t *testing.T) contract.Record {
	t.Helper()
	scope := contract.Scope{WorkspaceRoot: "/repo", CWD: "/repo", SourceRoot: "/repo", GitCommonDir: "/repo/.git"}
	actor := contract.NativeActor{
		Host: "codex", SessionID: "session",
		SessionProcess:  &issueopscontract.NativeProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/codex"},
		ProcessAncestry: []issueopscontract.NativeProcessReceipt{{PID: 1, StartedAt: "init", Executable: "/sbin/init"}},
	}
	key := domain.Key(scope, actor)
	return domain.NewRecord(key, scope, actor, domain.TokenDigest(domain.ComposeToken(key, "c2VjcmV0")), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
}

func TestRepositoryPersistsGrantInStateRootWithoutAncestry(t *testing.T) {
	root := t.TempDir()
	repository := Repository{StateRoot: root}
	if _, found, err := repository.Get(contract.Bucket, "missing"); err != nil || found {
		t.Fatalf("get before state exists found=%v err=%v", found, err)
	}
	if _, err := os.Stat(filepath.Join(root, "data.db")); err == nil {
		t.Fatal("an unspanned read created state")
	}
	record := grantFixture(t)
	record.Actor.ProcessAncestry = []issueopscontract.NativeProcessReceipt{{PID: 1, StartedAt: "init", Executable: "/sbin/init"}}
	var seen *contract.Record
	if err := repository.Within(context.Background(), record.Key, func(current *contract.Record) (*contract.Record, error) {
		seen = current
		return &record, nil
	}); err != nil {
		t.Fatal(err)
	}
	if seen != nil {
		t.Fatalf("first issue saw an existing grant: %+v", seen)
	}
	data, found, err := repository.Get(contract.Bucket, record.Key)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if strings.Contains(string(data), "init") {
		t.Fatalf("persisted grant contains ancestry: %s", data)
	}
	stored, err := domain.DecodeRecord(data, record.Key)
	if err != nil || stored.Actor.ProcessAncestry != nil {
		t.Fatalf("stored=%+v err=%v", stored, err)
	}
	if err := repository.Within(context.Background(), record.Key, func(current *contract.Record) (*contract.Record, error) {
		seen = current
		return nil, nil
	}); err != nil || seen == nil || seen.TokenSHA256 != record.TokenSHA256 {
		t.Fatalf("second span seen=%+v err=%v", seen, err)
	}
}

func TestRepositoryRejectsUnsupportedSchemaAndNestedLeaseSpan(t *testing.T) {
	root := t.TempDir()
	repository := Repository{StateRoot: root}
	record := grantFixture(t)
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	future := strings.Replace(string(mustEncode(t, record)), `"schema_version":1`, `"schema_version":2`, 1)
	if err := database.WithSpan(context.Background(), func(ctx context.Context) error {
		return database.Apply(ctx, []port.RecordMutation{{Bucket: contract.Bucket, ID: record.Key, Data: []byte(future)}})
	}); err != nil {
		t.Fatal(err)
	}
	called := false
	err = repository.Within(context.Background(), record.Key, func(*contract.Record) (*contract.Record, error) {
		called = true
		return &record, nil
	})
	if !errors.Is(err, contract.ErrInvalidState) || called {
		t.Fatalf("future grant err=%v callback=%v", err, called)
	}
	err = database.WithSpan(context.Background(), func(spanCtx context.Context) error {
		return repository.Within(spanCtx, record.Key, func(*contract.Record) (*contract.Record, error) { return &record, nil })
	})
	var nested *sqlstore.NestedSpanError
	if !errors.As(err, &nested) {
		t.Fatalf("issue inside a lease span must be refused as nested: %v", err)
	}
}

func mustEncode(t *testing.T, record contract.Record) []byte {
	t.Helper()
	data, err := domain.EncodeRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestScopeResolverSharesCommonDirAcrossWorktreesAndBoundsNonGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	worktree := filepath.Join(base, "repo.worktrees", "a")
	for _, args := range [][]string{
		{"init", "-q", repo},
		{"-C", repo, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"},
		{"-C", repo, "worktree", "add", "-q", worktree},
	} {
		if output, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, output)
		}
	}
	resolver := ScopeResolver{}
	main, err := resolver.Resolve(context.Background(), repo, "")
	if err != nil {
		t.Fatal(err)
	}
	tree, err := resolver.Resolve(context.Background(), worktree, filepath.Join(worktree, "."))
	if err != nil {
		t.Fatal(err)
	}
	if main.GitCommonDir == "" || main.GitCommonDir != tree.GitCommonDir || tree.SourceRoot == main.SourceRoot {
		t.Fatalf("main=%+v tree=%+v", main, tree)
	}
	plain := filepath.Join(base, "plain")
	if err := os.MkdirAll(filepath.Join(plain, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	scope, err := resolver.Resolve(context.Background(), plain, filepath.Join(plain, "sub"))
	if err != nil || scope.GitCommonDir != "" || !strings.HasSuffix(scope.SourceRoot, "plain") {
		t.Fatalf("non-git scope=%+v err=%v", scope, err)
	}
	for name, input := range map[string][2]string{
		"relative root":     {"plain", ""},
		"cwd outside root":  {plain, repo},
		"missing root":      {filepath.Join(base, "missing"), ""},
		"file as workspace": {filepath.Join(base, "file"), ""},
	} {
		if name == "file as workspace" {
			if err := os.WriteFile(input[0], nil, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := resolver.Resolve(context.Background(), input[0], input[1]); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
	t.Setenv("GIT_DIR", filepath.Join(repo, ".git"))
	scope, err = resolver.Resolve(context.Background(), plain, "")
	if err != nil || scope.GitCommonDir != "" {
		t.Fatalf("caller GIT_DIR must not redirect scope: %+v err=%v", scope, err)
	}
}
