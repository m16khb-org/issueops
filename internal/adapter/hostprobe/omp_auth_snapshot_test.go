package hostprobe

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/port"
)

func execOmpStatements(t *testing.T, path string, statements ...string) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSnapshotOmpAuthWithoutSourceCreatesNothing(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "agent.db")
	if err := snapshotOmpAuth(t.Context(), "", destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		t.Fatalf("destination created without source: %v", err)
	}
}

func TestSnapshotOmpAuthLeavesDestinationEmptyWhenSourceHasNoAuthTables(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	execOmpStatements(t, source, `CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT)`, `INSERT INTO settings VALUES ('k','ambient')`)
	destination := filepath.Join(dir, "agent.db")
	if err := snapshotOmpAuth(t.Context(), source, destination); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", "file:"+destination+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if tables := queryOmpStrings(t, db, `SELECT name FROM sqlite_master WHERE type='table'`); len(tables) != 0 {
		t.Fatalf("ambient tables leaked into snapshot: %q", tables)
	}
}

func TestSnapshotOmpAuthRejectsPartialAuthSchema(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	execOmpStatements(t, source, `CREATE TABLE auth_credentials (id INTEGER PRIMARY KEY, provider TEXT)`)
	if err := snapshotOmpAuth(t.Context(), source, filepath.Join(dir, "agent.db")); err == nil || err.Error() != "omp_auth_invalid" {
		t.Fatalf("err = %v", err)
	}
}

func TestSnapshotOmpAuthRefusesToOverwriteExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.db")
	writeOmpAuthStore(t, source)
	destination := filepath.Join(dir, "agent.db")
	if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := snapshotOmpAuth(t.Context(), source, destination); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	if data, _ := os.ReadFile(destination); string(data) != "existing" {
		t.Fatalf("destination modified: %q", data)
	}
}

func TestPrepareOmpEpisodeRejectsRelativePaths(t *testing.T) {
	request := ompProbeRequest()
	request.HarnessBinary = "/opt/bin/issueops"
	for name, tc := range map[string]struct {
		root, result string
		request      port.HostProbeRequest
	}{
		"relative root":   {"rel", "/r", request},
		"relative result": {t.TempDir(), "r", request},
		"relative binary": {t.TempDir(), "/r", func() port.HostProbeRequest { r := request; r.HarnessBinary = "issueops"; return r }()},
	} {
		if err := prepareOmpEpisode(tc.root, "ext", "tool", tc.request, tc.result); err == nil || err.Error() != "episode_path_invalid" {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	root := t.TempDir()
	if err := prepareOmpEpisode(root, "ext", "tool", request, "/r"); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(root, "sessions")); err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Fatalf("sessions dir: %v %v", info, err)
	}
}

func TestPrepareOmoEpisodeRejectsRelativePaths(t *testing.T) {
	request := omoProbeRequest()
	request.HarnessBinary = "issueops"
	if err := prepareOmoEpisode(t.TempDir(), "ext", "tool", request, "/r"); err == nil || err.Error() != "episode_path_invalid" {
		t.Fatalf("err = %v", err)
	}
}
