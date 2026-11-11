package issueopscli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"issueops/internal/testsupport"

	_ "modernc.org/sqlite"
)

// TestIssueOpsListJSONCarriesLastRecordAt는 실제 list 명령 경로가 lease claim
// 시각을 last_record_at으로 올리고 idle_days를 채우며, 그 조회가 state와 원격
// CLI를 건드리지 않음을 잠근다.
func TestIssueOpsListJSONCarriesLastRecordAt(t *testing.T) {
	root := seedIdleListState(t)
	buckets := snapshotBuckets(t, root)
	files := dirFileNames(t, root)

	var runErr error
	out := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"list", "--json"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops list --json: %v", runErr)
	}
	var result struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("list --json is not JSON: %v\n%s", err, out)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("entries = %d, want 1:\n%s", len(result.Entries), out)
	}
	entry := result.Entries[0]
	if got := entry["last_record_at"]; got != "2026-11-05T00:00:00Z" {
		t.Fatalf("last_record_at = %v, want the lease claim time:\n%s", got, out)
	}
	idle, ok := entry["idle_days"].(float64)
	if !ok || idle < 1 {
		t.Fatalf("idle_days = %v (present=%v), want >= 1:\n%s", entry["idle_days"], ok, out)
	}

	text := captureStdoutForContract(t, func() error {
		runErr = runIssueOps([]string{"list"})
		return runErr
	})
	if runErr != nil {
		t.Fatalf("issueops list: %v", runErr)
	}
	if !strings.Contains(text, "[idle:") {
		t.Fatalf("text list lacks the idle flag:\n%s", text)
	}

	if after := snapshotBuckets(t, root); !slices.Equal(after, buckets) {
		t.Fatalf("list changed the state records:\nbefore=%v\nafter=%v", buckets, after)
	}
	if after := dirFileNames(t, root); !slices.Equal(after, files) {
		t.Fatalf("list changed the state files:\nbefore=%v\nafter=%v", files, after)
	}
}

// TestIssueOpsListDoesNotInvokeRemoteCLIs는 list가 원격 CLI를 부르지 않음을
// 잠근다. fake 명령은 Windows에서 Git Bash가 필요하므로, 그것이 없을 때의
// skip이 위 테스트의 단정까지 건너뛰지 않도록 따로 둔다.
func TestIssueOpsListDoesNotInvokeRemoteCLIs(t *testing.T) {
	seedIdleListState(t)
	markers := t.TempDir()
	bin := t.TempDir()
	for _, tool := range []string{"gh", "glab"} {
		marker := filepath.ToSlash(filepath.Join(markers, tool+".called"))
		testsupport.WriteFakeCommand(t, bin, tool, "#!/bin/sh\n: > '"+marker+"'\nexit 1\n")
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for _, args := range [][]string{{"list", "--json"}, {"list"}} {
		var runErr error
		captureStdoutForContract(t, func() error {
			runErr = runIssueOps(args)
			return runErr
		})
		if runErr != nil {
			t.Fatalf("issueops %v: %v", args, runErr)
		}
	}
	for _, tool := range []string{"gh", "glab"} {
		if _, err := os.Stat(filepath.Join(markers, tool+".called")); !os.IsNotExist(err) {
			t.Fatalf("list invoked %s (stat err=%v)", tool, err)
		}
	}
}

// seedIdleListState는 임시 state에 UpdatedAt보다 lease claim이 늦은 record
// 하나를 넣고 state root를 돌려준다.
func seedIdleListState(t *testing.T) string {
	t.Helper()
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	root := issueOpsStateRootForTest()
	repo := testsupport.AbsPath("repo")
	seedListRecord(t, root, issueopscontract.Record{
		SchemaVersion: issueopscontract.SchemaVersion,
		ID:            "io-idle", Repo: repo, Phase: issueopscontract.PhaseProblem,
		UpdatedAt: "2026-11-01T00:00:00Z",
		Execution: &issueopscontract.Execution{
			Workspace: issueopscontract.Workspace{
				SourceRoot: repo, Root: testsupport.AbsPath("repo.worktrees", "io-idle"), Branch: "1-idle",
				BaseHead: "abc", LinkedAt: "2026-11-01T00:00:00Z",
			},
			Lease: issueopscontract.WriteLease{
				Generation:       1,
				Status:           issueopscontract.LeaseStatusClaimable,
				ClaimTokenSHA256: strings.Repeat("a", 64),
				ClaimedAt:        "2026-11-05T00:00:00Z",
			},
		},
	})
	return root
}

func seedListRecord(t *testing.T, root string, record issueopscontract.Record) {
	t.Helper()
	database, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := issueopsrecord.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	err = database.Apply(context.Background(), []port.RecordMutation{
		{Bucket: fmt.Sprintf("issueops_v%d", issueopscontract.SchemaVersion), ID: record.ID, Data: data},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// snapshotBuckets는 state DB의 모든 bucket에서 (bucket, id, sha256(data))
// 줄을 정렬해 돌려준다. 읽기 전용으로 열어 snapshot 자체가 state를 바꾸지 않는다.
func snapshotBuckets(t *testing.T, root string) []string {
	t.Helper()
	database, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(root, "issueops.db"))+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	rows, err := database.Query(`SELECT bucket, id, data FROM records ORDER BY bucket, id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	lines := []string{}
	for rows.Next() {
		var bucket, id string
		var data []byte
		if err := rows.Scan(&bucket, &id, &data); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		lines = append(lines, bucket+"/"+id+" "+hex.EncodeToString(sum[:]))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatal("snapshot saw no records; the seed did not land in this database")
	}
	return lines
}

func dirFileNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	return names
}
