package issueops

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"testing"
	"time"
)

// sleeperModeEnv makes the test binary (TestMain) sleep instead of running
// tests, a long-lived child with a chosen argv[0] where /bin/sleep is absent.
const sleeperModeEnv = "ISSUEOPS_TEST_SLEEPER"

func TestNativeProcessInspectorOpencodeArgumentsMatchStartIdentity(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
	if runtime.GOOS == "windows" {
		cmd = exec.Command(os.Args[0], "30")
		cmd.Env = append(os.Environ(), sleeperModeEnv+"=1")
	}
	cmd.Args = []string{"/opt/test/bin/opencode", "30"}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	receipt, err := ObserveNativeProcessReceipt(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	inspector := NativeProcessInspector{}
	fields, err := inspector.Arguments(context.Background(), receipt)
	if err != nil {
		t.Fatalf("arguments for live receipt: %v", err)
	}
	if want := []string{"/opt/test/bin/opencode", "30"}; !slices.Equal(fields, want) {
		t.Fatalf("arguments = %q, want %q", fields, want)
	}

	reused := receipt
	started, err := time.Parse(time.RFC3339, receipt.StartedAt)
	if err != nil {
		t.Fatal(err)
	}
	reused.StartedAt = started.Add(-time.Second).UTC().Format(time.RFC3339)
	if fields, err := inspector.Arguments(context.Background(), reused); err == nil {
		t.Fatalf("arguments returned for a mismatched start identity: %q", fields)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := inspector.Arguments(canceled, receipt); err == nil {
		t.Fatal("arguments ignored a canceled context")
	}
}
