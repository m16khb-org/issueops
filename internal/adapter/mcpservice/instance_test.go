package mcpservice

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/processidentity"
)

func writeBinary(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "issueops")
	if err := os.WriteFile(path, []byte("binary-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPublishWritesRecordThatReleaseRemoves(t *testing.T) {
	state := t.TempDir()
	binary := writeBinary(t)
	instance, err := AcquireInstance(state)
	if err != nil {
		t.Fatal(err)
	}
	inspect := func(pid int) (processidentity.Identity, error) {
		if pid != os.Getpid() {
			t.Errorf("inspected pid %d, want own pid", pid)
		}
		return processidentity.Identity{StartTime: "2026-10-02T00:00:00Z", Executable: "/bin/issueops"}, nil
	}
	record, err := instance.Publish(inspect, binary)
	if err != nil {
		t.Fatal(err)
	}
	wantBuild, err := BuildID(binary)
	if err != nil {
		t.Fatal(err)
	}
	if record.PID != os.Getpid() || record.StartedAt != "2026-10-02T00:00:00Z" || record.Executable != "/bin/issueops" || record.BuildID != wantBuild {
		t.Fatalf("record = %+v", record)
	}
	onDisk, err := readRecord(filepath.Join(httpDir(state), recordFileName))
	if err != nil || onDisk != record {
		t.Fatalf("on disk = %+v, %v; want %+v", onDisk, err, record)
	}
	if err := instance.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(httpDir(state), recordFileName)); !os.IsNotExist(err) {
		t.Fatalf("record survived release: %v", err)
	}
}

func TestReleaseKeepsRecordOwnedByAnotherProcess(t *testing.T) {
	state := t.TempDir()
	instance, err := AcquireInstance(state)
	if err != nil {
		t.Fatal(err)
	}
	inspect := func(int) (processidentity.Identity, error) {
		return processidentity.Identity{StartTime: "t", Executable: "/bin/issueops"}, nil
	}
	record, err := instance.Publish(inspect, writeBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(httpDir(state), recordFileName)
	successor := record
	successor.PID = record.PID + 1
	if err := writeRecord(path, successor); err != nil {
		t.Fatal(err)
	}
	if err := instance.Release(); err != nil {
		t.Fatal(err)
	}
	kept, err := readRecord(path)
	if err != nil || kept.PID != successor.PID {
		t.Fatalf("a successor's record was removed: %+v, %v", kept, err)
	}
}

func TestReleaseWithoutPublishLeavesExistingRecord(t *testing.T) {
	state := t.TempDir()
	instance, err := AcquireInstance(state)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(httpDir(state), recordFileName)
	existing := Record{SchemaVersion: recordSchema, PID: 77, StartedAt: "t", Executable: "/bin/x", BuildID: "b"}
	if err := writeRecord(path, existing); err != nil {
		t.Fatal(err)
	}
	if err := instance.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("unpublished instance removed someone else's record: %v", err)
	}
}

func TestPublishFailuresLeaveNoRecord(t *testing.T) {
	for _, tc := range []struct {
		name       string
		inspectErr bool
		executable func(t *testing.T) string
		want       string
	}{
		{name: "identity cannot be inspected", inspectErr: true, executable: writeBinary, want: "inspect mcp http process"},
		{name: "executable cannot be hashed", executable: func(t *testing.T) string { return filepath.Join(t.TempDir(), "missing") }, want: "mcp http build id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := t.TempDir()
			instance, err := AcquireInstance(state)
			if err != nil {
				t.Fatal(err)
			}
			defer instance.Release()
			inspect := func(int) (processidentity.Identity, error) {
				if tc.inspectErr {
					return processidentity.Identity{}, os.ErrPermission
				}
				return processidentity.Identity{StartTime: "t", Executable: "/bin/x"}, nil
			}
			if _, err := instance.Publish(inspect, tc.executable(t)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Publish error = %v, want %q", err, tc.want)
			}
			if _, err := os.Stat(filepath.Join(httpDir(state), recordFileName)); !os.IsNotExist(err) {
				t.Fatalf("failed publish left a record: %v", err)
			}
		})
	}
}

func TestAcquireInstanceFailsWhenStateDirIsNotADirectory(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "state")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireInstance(blocker); err == nil {
		t.Fatal("AcquireInstance succeeded under a regular file")
	}
}

func TestReadRecordRejectsMalformedOrIncompleteRecords(t *testing.T) {
	valid := Record{SchemaVersion: recordSchema, PID: 5, StartedAt: "t", Executable: "/bin/x", BuildID: "b"}
	for _, tc := range []struct {
		name   string
		mutate func(*Record)
		raw    string
	}{
		{name: "not JSON", raw: "{broken"},
		{name: "wrong schema", mutate: func(r *Record) { r.SchemaVersion = 2 }},
		{name: "zero pid", mutate: func(r *Record) { r.PID = 0 }},
		{name: "negative pid", mutate: func(r *Record) { r.PID = -3 }},
		{name: "empty started_at", mutate: func(r *Record) { r.StartedAt = "" }},
		{name: "empty executable", mutate: func(r *Record) { r.Executable = "" }},
		{name: "empty build id", mutate: func(r *Record) { r.BuildID = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), recordFileName)
			if tc.raw != "" {
				if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
					t.Fatal(err)
				}
			} else {
				record := valid
				tc.mutate(&record)
				if err := writeRecord(path, record); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := readRecord(path); err == nil {
				t.Fatal("readRecord accepted an invalid record")
			}
		})
	}
	path := filepath.Join(t.TempDir(), recordFileName)
	if err := writeRecord(path, valid); err != nil {
		t.Fatal(err)
	}
	if got, err := readRecord(path); err != nil || got != valid {
		t.Fatalf("valid record = %+v, %v", got, err)
	}
}

func TestWriteRecordReplacesAtomicallyWithOwnerOnlyMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), recordFileName)
	record := Record{SchemaVersion: recordSchema, PID: 5, StartedAt: "t", Executable: "/bin/x", BuildID: "b"}
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	record.PID = 6
	if err := writeRecord(path, record); err != nil {
		t.Fatal(err)
	}
	if got, err := readRecord(path); err != nil || got.PID != 6 {
		t.Fatalf("replaced record = %+v, %v", got, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temporary record left behind: %v", err)
	}
	if err := writeRecord(filepath.Join(t.TempDir(), "missing", recordFileName), record); err == nil {
		t.Fatal("writeRecord succeeded into a missing directory")
	}
}

func TestReadBearerTrimsSurroundingWhitespaceOnly(t *testing.T) {
	state := t.TempDir()
	if err := os.MkdirAll(httpDir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(httpDir(state), bearerFileName)
	if err := os.WriteFile(path, []byte(" \t\r\nab cd\r\n\t "), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadBearer(state); err != nil || got != "ab cd" {
		t.Fatalf("bearer = %q, %v", got, err)
	}
	if err := os.WriteFile(path, []byte(" \n\t"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadBearer(state); err != nil || got != "" {
		t.Fatalf("whitespace-only bearer = %q, %v", got, err)
	}
}

func TestReadBearerRejectsSymlinkAndUnreadableStateDir(t *testing.T) {
	state := t.TempDir()
	if err := os.MkdirAll(httpDir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(httpDir(state), bearerFileName)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBearer(state); err == nil {
		t.Fatal("bearer symlink accepted")
	}
	notDir := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDir, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadBearer(notDir); err == nil {
		t.Fatal("lstat failure other than not-exist was swallowed")
	}
}

func TestIdentityHandlerStampsOnlyFirstStatusAndExposesUnderlyingWriter(t *testing.T) {
	record := Record{PID: 9, BuildID: "build-9"}
	var inner http.ResponseWriter
	handler := IdentityHandler(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		rw.WriteHeader(http.StatusForbidden)
		rw.WriteHeader(http.StatusOK)
		inner = rw.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
	}), record)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if rec.Header().Get(HeaderPID) != "" {
		t.Fatalf("a later 200 stamped identity onto a 403: %v", rec.Header())
	}
	if inner != http.ResponseWriter(rec) {
		t.Fatalf("Unwrap returned %T, want the underlying recorder", inner)
	}
}

func TestIdentityHandlerStampsOnlySuccessStatuses(t *testing.T) {
	record := Record{PID: 9, BuildID: "build-9"}
	for status, want := range map[int]bool{
		http.StatusOK:                  true,
		http.StatusNoContent:           true,
		299:                            true,
		http.StatusMultipleChoices:     false,
		http.StatusInternalServerError: false,
	} {
		handler := IdentityHandler(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
			rw.WriteHeader(status)
		}), record)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
		pid, build := rec.Header().Get(HeaderPID), rec.Header().Get(HeaderBuildID)
		if want && (pid != "9" || build != "build-9") {
			t.Fatalf("status %d must carry both identity headers: %v", status, rec.Header())
		}
		if !want && (pid != "" || build != "") {
			t.Fatalf("status %d must not leak either identity header: %v", status, rec.Header())
		}
	}
}
