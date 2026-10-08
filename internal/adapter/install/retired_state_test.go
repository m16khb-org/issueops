package install

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeRetiredFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveRetiredStateDeletesOnlyAllowlistedEntries(t *testing.T) {
	state := t.TempDir()
	outside := filepath.Join(t.TempDir(), "receipt.json")
	writeRetiredFixture(t, outside, "user data")
	for _, name := range []string{"daemon/issueops.log", "hook-failures.jsonl", "hook-metrics.jsonl", ".last-store-maintain", "keep.json", "channel/issueops.db"} {
		writeRetiredFixture(t, filepath.Join(state, name), "x")
	}
	if err := os.Symlink(outside, filepath.Join(state, "issueops-migration-receipt.json")); err != nil {
		t.Fatal(err)
	}
	removable := []string{".last-store-maintain", "daemon", "hook-failures.jsonl", "hook-metrics.jsonl"}

	planned := strings.Join(RemoveRetiredState(state, true), "\n")
	for _, name := range removable {
		if !strings.Contains(planned, "would remove retired state path "+filepath.Join(state, name)) || !exists(filepath.Join(state, name)) {
			t.Fatalf("dry run for %s: messages=%s", name, planned)
		}
	}

	messages := strings.Join(RemoveRetiredState(state, false), "\n")
	for _, name := range removable {
		if exists(filepath.Join(state, name)) || !strings.Contains(messages, "removed retired state path "+filepath.Join(state, name)) {
			t.Fatalf("%s not removed: messages=%s", name, messages)
		}
	}
	for _, name := range []string{"keep.json", "channel/issueops.db", "issueops-migration-receipt.json"} {
		if !exists(filepath.Join(state, name)) {
			t.Fatalf("%s removed", name)
		}
	}
	if data, err := os.ReadFile(outside); err != nil || string(data) != "user data" {
		t.Fatalf("symlink target touched: %q %v", data, err)
	}
	if !strings.Contains(messages, filepath.Join(state, "issueops-migration-receipt.json")+" is not a regular file") {
		t.Fatalf("symlink skip not reported: %s", messages)
	}
	if again := RemoveRetiredState(state, false); len(again) != 1 {
		t.Fatalf("second run messages = %v", again)
	}
}

func TestRemoveRetiredStateKeepsDaemonWhileLegacyProcessIsAlive(t *testing.T) {
	exited := exec.Command("true")
	if err := exited.Run(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		pid    int
		socket bool
		keep   bool
	}{
		{name: "live pid", pid: os.Getpid(), keep: true},
		{name: "listening socket", socket: true, keep: true},
		{name: "exited pid", pid: exited.Process.Pid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, err := os.MkdirTemp("/tmp", "retired")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(state) })
			daemon := filepath.Join(state, "daemon")
			writeRetiredFixture(t, filepath.Join(daemon, "issueops.log"), "log")
			if tc.pid != 0 {
				writeRetiredFixture(t, filepath.Join(daemon, "issueops.pid"), fmt.Sprintf(`{"pid":%d,"executable":"/old/issueops"}`, tc.pid))
			}
			if tc.socket {
				listener, err := net.Listen("unix", filepath.Join(daemon, "issueops.sock"))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = listener.Close() })
			}
			messages := strings.Join(RemoveRetiredState(state, false), "\n")
			if exists(daemon) != tc.keep {
				t.Fatalf("daemon kept=%v want %v: %s", exists(daemon), tc.keep, messages)
			}
			if tc.keep && !strings.Contains(messages, "legacy daemon") {
				t.Fatalf("live daemon skip not reported: %s", messages)
			}
		})
	}
}
