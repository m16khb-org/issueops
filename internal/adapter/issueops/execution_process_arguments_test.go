package issueops

import (
	"context"
	"os/exec"
	"slices"
	"testing"
	"time"
)

func TestNativeProcessInspectorOpencodeArgumentsMatchStartIdentity(t *testing.T) {
	cmd := exec.Command("/bin/sleep", "30")
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
