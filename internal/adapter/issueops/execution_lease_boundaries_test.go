package issueops

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// fakeSnapshotGit는 LeaseWorkspaceSnapshot에 주입하는 가짜 git이다. 명령별 응답과
// 실패를 고르게 하고, 프로세스를 띄우지 않아 결정적이다.
type fakeSnapshotGit struct {
	root      string
	branch    string
	head      string
	index     string
	untracked string
	fail      map[string]bool
}

func (g *fakeSnapshotGit) run(args ...string) (int, string, string) {
	key := strings.Join(args, " ")
	if g.fail[key] {
		return 1, "", "boom"
	}
	switch key {
	case "rev-parse --show-toplevel":
		return 0, g.root + "\n", ""
	case "branch --show-current":
		return 0, g.branch + "\n", ""
	case "rev-parse HEAD":
		return 0, g.head + "\n", ""
	case "rev-parse --git-common-dir":
		return 0, ".git\n", ""
	case "rev-parse --git-path index":
		return 0, g.index + "\n", ""
	case "diff --binary --no-ext-diff --", "diff --cached --binary --no-ext-diff --":
		return 0, "", ""
	case "ls-files --others --exclude-standard -z":
		return 0, g.untracked, ""
	}
	return 1, "", "unexpected git " + key
}

func (g *fakeSnapshotGit) snapshot() (string, error) {
	cmd := func(_ string, args ...string) (int, string, string) { return g.run(args...) }
	return LeaseWorkspaceSnapshot{GitCmd: cmd, GitCmdRaw: cmd}.Snapshot(issueops.Workspace{Root: g.root, Branch: g.branch})
}

func newFakeSnapshotGit(t *testing.T) *fakeSnapshotGit {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &fakeSnapshotGit{root: root, branch: "feat", head: "abc123", index: filepath.Join(root, "idx"), fail: map[string]bool{}}
}

func TestLeaseWorkspaceSnapshotSealsCleanAndUntrackedWorktreeState(t *testing.T) {
	g := newFakeSnapshotGit(t)
	if err := os.WriteFile(g.index, []byte("index-v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	clean, err := g.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(clean) {
		t.Fatalf("snapshot = %q, want a sha256 hex digest", clean)
	}
	again, err := g.snapshot()
	if err != nil || again != clean {
		t.Fatalf("repeat snapshot = %q, %v; want stable %q", again, err, clean)
	}

	g.head = "def456"
	if moved, err := g.snapshot(); err != nil || moved == clean {
		t.Fatalf("HEAD move snapshot = %q, %v; want a different seal", moved, err)
	}
	g.head = "abc123"

	for name, content := range map[string]string{"a.txt": "one", "b.txt": "two"} {
		if err := os.WriteFile(filepath.Join(g.root, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g.untracked = "a.txt\x00b.txt\x00"
	withFiles, err := g.snapshot()
	if err != nil || withFiles == "" || withFiles == clean {
		t.Fatalf("untracked snapshot = %q, %v; want non-empty seal differing from clean", withFiles, err)
	}
	// 두 번째 untracked 파일도 봉인에 반영되어야 한다.
	if err := os.WriteFile(filepath.Join(g.root, "b.txt"), []byte("TWO"), 0o644); err != nil {
		t.Fatal(err)
	}
	if edited, err := g.snapshot(); err != nil || edited == withFiles {
		t.Fatalf("second untracked file edit = %q, %v; want a different seal", edited, err)
	}
}

func TestLeaseWorkspaceSnapshotSurfacesGitFailures(t *testing.T) {
	for _, tc := range []struct {
		name string
		fail string
		want string
	}{
		{"top-level", "rev-parse --show-toplevel", "does not match Git top-level"},
		{"branch", "branch --show-current", "branch mismatch"},
		{"head", "rev-parse HEAD", "git rev-parse HEAD"},
		{"common dir", "rev-parse --git-common-dir", "git rev-parse --git-common-dir"},
		{"index path", "rev-parse --git-path index", "git rev-parse --git-path index"},
		{"tracked diff", "diff --binary --no-ext-diff --", "read tracked diff"},
		{"staged diff", "diff --cached --binary --no-ext-diff --", "read staged diff"},
		{"untracked list", "ls-files --others --exclude-standard -z", "list untracked files"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newFakeSnapshotGit(t)
			g.fail[tc.fail] = true
			snap, err := g.snapshot()
			if err == nil || !strings.Contains(err.Error(), tc.want) || snap != "" {
				t.Fatalf("snapshot = %q, %v; want error containing %q", snap, err, tc.want)
			}
		})
	}
}

func TestLeaseWorkspaceSnapshotRefusesUnreadableIndexButToleratesMissingOne(t *testing.T) {
	g := newFakeSnapshotGit(t)
	if snap, err := g.snapshot(); err != nil || snap == "" {
		t.Fatalf("missing index snapshot = %q, %v; want success", snap, err)
	}
	if err := os.Mkdir(g.index, 0o755); err != nil {
		t.Fatal(err)
	}
	snap, err := g.snapshot()
	if err == nil || !strings.Contains(err.Error(), "is a directory") || snap != "" {
		t.Fatalf("directory index snapshot = %q, %v; want the read error", snap, err)
	}
}

func TestLeaseWorkspaceSnapshotRefusesUnsafeUntrackedEntries(t *testing.T) {
	g := newFakeSnapshotGit(t)
	g.untracked = "../escape.txt\x00"
	snap, err := g.snapshot()
	if err == nil || !strings.Contains(err.Error(), "unsafe untracked path") || snap != "" {
		t.Fatalf("escaping path snapshot = %q, %v", snap, err)
	}

	g.untracked = "missing.txt\x00"
	snap, err = g.snapshot()
	if err == nil || !strings.Contains(err.Error(), "must be a regular file") || snap != "" {
		t.Fatalf("missing untracked file snapshot = %q, %v", snap, err)
	}
}

func TestRolloverInspectionUsesTheProvidedProcessSnapshotWhenPresent(t *testing.T) {
	t.Parallel()

	pid := os.Getpid()
	receipt := issueops.NativeProcessReceipt{PID: pid, StartedAt: "snapshot-start", Executable: "/snapshot/exe"}
	snapshot := map[int]nativeProcessSnapshotEntry{pid: {Receipt: receipt}}

	status, observed, err := replacementProcessSnapshot{entries: snapshot}.Inspect(receipt)
	if err != nil || status != NativeProcessStatusLive || observed != receipt {
		t.Fatalf("snapshot inspection = %q, %+v, %v; want live from the snapshot", status, observed, err)
	}
	// 스냅샷이 없으면 실제 프로세스를 관측하므로 가짜 receipt는 신원 불일치다.
	status, _, err = replacementProcessSnapshot{}.Inspect(receipt)
	if err != nil || status != NativeProcessStatusIdentityMismatch {
		t.Fatalf("live inspection = %q, %v; want %q", status, err, NativeProcessStatusIdentityMismatch)
	}
}

func TestListLeaseHolderIndexesAcceptsEverySupportedHostAndRejectsOthers(t *testing.T) {
	t.Parallel()

	put := func(t *testing.T, host string) (string, error) {
		t.Helper()
		stateRoot := t.TempDir()
		db, err := sqlstore.Open(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		actor := issueops.NativeActor{Host: host, SessionID: "session-" + host}
		data, err := json.Marshal(issueops.LeaseHolderIndex{
			SchemaVersion: issueops.IssueOpsSchemaVersion, LifecycleID: "io-0123456789ab", Generation: 1,
			Host: host, SessionID: actor.SessionID,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: leaseHolderBucket, ID: leaseHolderIndexKey(actor), Data: data}}); err != nil {
			t.Fatal(err)
		}
		rows, err := ListLeaseHolderIndexes(stateRoot)
		if err != nil {
			return "", err
		}
		if len(rows) != 1 {
			t.Fatalf("host %q rows = %#v", host, rows)
		}
		return rows[0].Host, nil
	}
	for _, host := range []string{"codex", "claude", "omo", "omp", "opencode"} {
		if got, err := put(t, host); err != nil || got != host {
			t.Errorf("host %q: got %q, %v; want accepted", host, got, err)
		}
	}
	for _, host := range []string{"gemini", ""} {
		if _, err := put(t, host); err == nil || !strings.Contains(err.Error(), "is invalid") {
			t.Errorf("host %q: err = %v; want invalid index", host, err)
		}
	}
}

func TestPersistExecutionTransitionIndexesTheActiveLeaseHolder(t *testing.T) {
	t.Parallel()

	_, record := replacementExecutionFixture(t)
	stateRoot := t.TempDir()
	if _, err := (ReplacementRecords{StateRoot: stateRoot}).Persist(context.Background(), record, nil); err != nil {
		t.Fatal(err)
	}
	rows, err := ListLeaseHolderIndexes(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	holder := record.Execution.Lease.Holder
	if len(rows) != 1 || rows[0].LifecycleID != record.ID || rows[0].Generation != record.Execution.Lease.Generation ||
		rows[0].Host != holder.Host || rows[0].SessionID != holder.SessionID || rows[0].Key != leaseHolderIndexKey(*holder) {
		t.Fatalf("lease holder indexes = %#v, want one for %s", rows, record.ID)
	}
}

func TestPersistExecutionTransitionRefusesHolderOfAnotherActiveLease(t *testing.T) {
	t.Parallel()

	_, record := replacementExecutionFixture(t)
	stateRoot := t.TempDir()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	holder := *record.Execution.Lease.Holder
	data, err := json.Marshal(issueops.LeaseHolderIndex{
		SchemaVersion: issueops.IssueOpsSchemaVersion, LifecycleID: "io-other0000000", Generation: 7,
		Host: holder.Host, SessionID: holder.SessionID, AgentID: holder.AgentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: leaseHolderBucket, ID: leaseHolderIndexKey(holder), Data: data}}); err != nil {
		t.Fatal(err)
	}
	_, err = ReplacementRecords{StateRoot: stateRoot}.Persist(context.Background(), record, nil)
	if err == nil || !strings.Contains(err.Error(), "native session already holds active lease io-other0000000 generation 7") {
		t.Fatalf("err = %v, want refusal naming the other lease", err)
	}
	if stored, readErr := ReadIssueOps(stateRoot, record.ID); readErr == nil {
		t.Fatalf("record was persisted despite the refusal: %+v", stored.Execution)
	}
}

func writeClaimTokenOfSize(t *testing.T, record issueops.IssueOpsRecord, size int) {
	t.Helper()
	path := claimTokenPath(record)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("a", size)), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReplacementCleanupRemovesTokenResidue(t *testing.T) {
	t.Parallel()

	record := claimTokenRecord(t)
	writeClaimTokenOfSize(t, record, 8)
	if err := (ReplacementArtifacts{}).Cleanup(record); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(claimTokenPath(record)); !os.IsNotExist(err) {
		t.Errorf("claim token survived replacement cleanup: %v", err)
	}
}

func TestReplacementCleanupReportsAResidueThatCannotBeRemoved(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	record := claimTokenRecord(t)
	token := claimTokenPath(record)
	if err := os.MkdirAll(filepath.Dir(token), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("residue"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(token)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	err := (ReplacementArtifacts{}).Cleanup(record)
	if err == nil || !strings.Contains(err.Error(), "clean uncommitted replacement residue") {
		t.Fatalf("err = %v, want cleanup failure", err)
	}
	if _, statErr := os.Lstat(token); statErr != nil {
		t.Fatalf("token should remain after failed removal: %v", statErr)
	}
}
