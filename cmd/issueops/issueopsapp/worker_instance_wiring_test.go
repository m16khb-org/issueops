package issueopsapp

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/internal/adapter/outbound/sqlstore"
	workercontract "issueops/internal/contract/worker"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerMCPInstancesKeepSameIDInSeparateStores(t *testing.T) {
	var deps [2]mcpcli.MCPDependencies
	var sessions [2]*mcp.ClientSession
	var directories [2]string
	for i, payload := range []string{"first store", "second store"} {
		directories[i] = filepath.Join(t.TempDir(), "worker")
		t.Setenv("ISSUEOPS_WORKER_DIR", directories[i])
		deps[i] = issueOpsMCPDependencies()
		sessions[i] = startHistoryMCPTestSession(t, deps[i])
		db, err := sqlstore.Open(directories[i])
		if err != nil {
			t.Fatal(err)
		}
		job := workercontract.WorkerJob{OK: true, ID: "same-id", Kind: "fixture", Payload: payload, Status: "queued", NoShell: true, CreatedAt: "2026-01-01T00:00:00Z", UpdatedAt: "2026-01-01T00:00:00Z"}
		data, err := json.Marshal(job)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put("worker", job.ID, data); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("ISSUEOPS_WORKER_DIR", directories[0])
	call := func(i int, sdk bool, name string, args map[string]any, out any) {
		t.Helper()
		var raw string
		if sdk {
			result, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil || result.IsError {
				t.Fatalf("sdk %d %s: %+v %v", i, name, result, err)
			}
			raw = result.Content[0].(*mcp.TextContent).Text
		} else {
			params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
			if err != nil {
				t.Fatal(err)
			}
			result, callErr := callSDKTool(t, params, deps[i])
			if callErr != nil {
				t.Fatal(callErr)
			}
			raw = result.(map[string]any)["content"].([]map[string]any)[0]["text"].(string)
		}
		if err := json.Unmarshal([]byte(raw), out); err != nil {
			t.Fatal(err)
		}
	}
	for _, sdk := range []bool{false, true} {
		for i, want := range []string{"first store", "second store"} {
			var job workercontract.WorkerJob
			call(i, sdk, "worker_status", map[string]any{"id": "same-id"}, &job)
			if job.Payload != want || job.WorkerDir != directories[i] {
				t.Fatalf("instance %d sdk=%t leaked: %+v", i, sdk, job)
			}
		}
	}
	var cancelled workercontract.WorkerJob
	call(0, true, "worker_cancel", map[string]any{"id": "same-id"}, &cancelled)
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancel: %+v", cancelled)
	}
	for i, want := range []string{"cancelled", "queued"} {
		for _, sdk := range []bool{false, true} {
			var list workercontract.WorkerListResult
			call(i, sdk, "worker_list", map[string]any{}, &list)
			if len(list.Jobs) != 1 || list.Jobs[0].Status != want || list.Queue == nil || list.Queue.Total != 1 {
				t.Fatalf("instance %d sdk=%t list: %+v", i, sdk, list)
			}
		}
	}
}

func TestWorkerCLIKeepsCapturedRelativeStateAndDefaultCWD(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	t.Setenv("ISSUEOPS_WORKER_DIR", "")
	t.Setenv("ISSUEOPS_STATE_DIR", "relative-state")
	if err := os.WriteFile("note.txt", []byte("captured cwd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := newWorkerCommand()
	expectedDB := filepath.Join(base, "relative-state", "worker", "issueops.db")
	if _, err := os.Stat(expectedDB); !os.IsNotExist(err) {
		t.Fatalf("construction wrote state: %v", err)
	}
	other := t.TempDir()
	t.Chdir(other)
	wrong := filepath.Join(other, "wrong-worker")
	t.Setenv("ISSUEOPS_WORKER_DIR", wrong)
	raw := captureStdoutForContract(t, func() error { return command.Run([]string{"run", "--read-only", "--json", "--", "cat", "note.txt"}) })
	var job workercontract.WorkerJob
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		t.Fatal(err)
	}
	if job.WorkerDir != filepath.Join("relative-state", "worker") || job.Status != "succeeded" || job.Result == nil || job.Result.Stdout != "captured cwd\n" {
		t.Fatalf("captured command: %+v", job)
	}
	stored, err := command.Service.Read(job.ID)
	if err != nil || stored.Status != "succeeded" {
		t.Fatalf("captured read: %+v %v", stored, err)
	}
	if _, err := os.Stat(expectedDB); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{wrong, filepath.Join(other, "relative-state")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("wrote changed context %s: %v", path, err)
		}
	}
}

func TestWorkerInstancesShareFullStoreSpanLock(t *testing.T) {
	t.Setenv("ISSUEOPS_WORKER_DIR", t.TempDir())
	first, second := newWorkerStore(), newWorkerStore()
	entered, release, attempted, finished := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan error, 1)
	defer close(release)
	go func() {
		finished <- first.WithLock(context.Background(), first.Directory, "first", func(context.Context) error { close(entered); <-release; return nil })
	}()
	<-entered
	secondEntered := make(chan struct{}, 1)
	done := make(chan error, 1)
	go func() {
		close(attempted)
		done <- second.WithLock(context.Background(), second.Directory, "second", func(context.Context) error { secondEntered <- struct{}{}; return nil })
	}()
	<-attempted
	select {
	case <-secondEntered:
		t.Error("second instance entered an active store span")
	case <-time.After(40 * time.Millisecond):
	}
	// Release via a separate signal so both goroutines finish before the test ends.
	release <- struct{}{}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestWorkerCapturedDirectoryPreservesRelativeMkdirError(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("ISSUEOPS_WORKER_DIR", "")
	t.Setenv("ISSUEOPS_STATE_DIR", "relative-state")
	if err := os.WriteFile("relative-state", []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected := os.MkdirAll(filepath.Join("relative-state", "worker"), 0o700)
	if expected == nil {
		t.Fatal("fixture must block worker directory creation")
	}
	service := newWorkerService()
	t.Chdir(t.TempDir())
	_, err := service.Enqueue(context.Background(), "fixture", "")
	if err == nil || err.Error() != expected.Error() {
		t.Fatalf("mkdir error changed: %v want %v", err, expected)
	}
}
