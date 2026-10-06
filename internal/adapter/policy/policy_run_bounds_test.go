package policy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	policycontract "issueops/internal/contract/policy"
)

// Helpers use argv instead of ambient env: the executor intentionally clears env.
func TestPolicyExecutionHelper(t *testing.T) {
	args := os.Args
	for len(args) > 0 && args[0] != "policy-execution-helper" {
		args = args[1:]
	}
	if len(args) == 0 {
		return
	}
	args = args[1:]
	switch args[0] {
	case "flood":
		size, _ := strconv.Atoi(args[1])
		var wg sync.WaitGroup
		for _, w := range []io.Writer{os.Stdout, os.Stderr} {
			wg.Go(func() {
				chunk := strings.Repeat("x", 8191) + "\n"
				for n := 0; n < size; n += len(chunk) {
					_, _ = io.WriteString(w, chunk)
				}
			})
		}
		wg.Wait()
	case "child":
		_ = os.WriteFile(args[1]+".ready", []byte("ready"), 0600)
		time.Sleep(2 * time.Second)
		_ = os.WriteFile(args[1], []byte("completed"), 0600)
	case "parent", "early-parent":
		child := exec.Command(os.Args[0], "-test.run=^TestPolicyExecutionHelper$", "policy-execution-helper", "child", args[1])
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		deadline := time.Now().Add(time.Second)
		for !existsForTest(args[1]+".ready") && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		fmt.Println("child ready")
		if args[0] == "parent" {
			_ = child.Wait()
		}
	case "exit":
		fmt.Fprint(os.Stdout, "ok\n")
		fmt.Fprint(os.Stderr, "diagnostic\n")
		code, _ := strconv.Atoi(args[1])
		os.Exit(code)
	}
	os.Exit(0)
}

func helperRequest(t *testing.T, mode string, args ...string) policycontract.CommandPolicyRequest {
	t.Helper()
	argv := []string{os.Args[0], "-test.run=^TestPolicyExecutionHelper$", "policy-execution-helper", mode}
	return policycontract.CommandPolicyRequest{CWD: t.TempDir(), Argv: append(argv, args...)}
}

func TestCommandExecutorTimeoutStopsInheritedPipesAndChildren(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("process-group guarantee is Unix-only")
	}
	for _, mode := range []string{"parent", "early-parent"} {
		t.Run(mode, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "completed")
			req := helperRequest(t, mode, marker)
			start := time.Now()
			result := (CommandExecutor{}).Execute(req, 750*time.Millisecond)
			elapsed := time.Since(start)
			if !strings.Contains(result.Stdout, "child ready") {
				t.Fatalf("child handshake missing: %v", result.Err)
			}
			if !result.TimedOut || result.Err == nil || elapsed > 1250*time.Millisecond {
				t.Errorf("timeout=%v err=%v elapsed=%v", result.TimedOut, result.Err, elapsed)
			}
			// Wait past the child's planned write so absence proves termination, not timing.
			if delay := 2500*time.Millisecond - time.Since(start); delay > 0 {
				time.Sleep(delay)
			}
			if existsForTest(marker) {
				t.Error("child survived timeout and wrote completion marker")
			}
		})
	}
}

func TestCommandExecutorCaptureIsBoundedWhileDrainingBothStreams(t *testing.T) {
	for _, size := range []int{8 << 20, 32 << 20} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			req := helperRequest(t, "flood", strconv.Itoa(size))
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			started := time.Now()
			result := (CommandExecutor{}).Execute(req, 10*time.Second)
			runtime.ReadMemStats(&after)
			t.Logf("size=%d retained=%d alloc=%d elapsed=%v", size, len(result.Stdout)+len(result.Stderr), after.TotalAlloc-before.TotalAlloc, time.Since(started))
			if result.Err != nil || result.ExitCode != 0 || result.TimedOut {
				t.Fatalf("flood execution failed: %v", result.Err)
			}
			if len(result.Stdout) > 64<<10 || len(result.Stderr) > 64<<10 {
				t.Errorf("unbounded capture: stdout=%d stderr=%d", len(result.Stdout), len(result.Stderr))
			}
			// A generous budget distinguishes fixed capture from output-sized copies.
			if after.TotalAlloc-before.TotalAlloc > 4<<20 {
				t.Errorf("capture allocations scale with output: %d", after.TotalAlloc-before.TotalAlloc)
			}
		})
	}
}

func TestCommandExecutorPreservesExitAndStartFailure(t *testing.T) {
	for _, code := range []int{0, 7} {
		result := (CommandExecutor{}).Execute(helperRequest(t, "exit", strconv.Itoa(code)), 5*time.Second)
		if result.ExitCode != code || result.TimedOut || result.Stdout != "ok\n" || result.Stderr != "diagnostic\n" || (result.Err != nil) != (code != 0) {
			t.Fatalf("exit contract: %+v", result)
		}
	}
	result := (CommandExecutor{}).Execute(policycontract.CommandPolicyRequest{Argv: []string{filepath.Join(t.TempDir(), "missing")}}, time.Second)
	if result.Err == nil || result.ExitCode != 1 || result.TimedOut {
		t.Fatalf("start failure: %+v", result)
	}
}

func TestBoundedOutputDropsUnexaminedSecretTail(t *testing.T) {
	for _, tail := range []string{
		"token=fake-value", "password=fake-value", "credential=fake-value",
		"ghp_" + strings.Repeat("a", 40), "https://fake-user:fake-password@example.invalid/path",
		strings.Repeat("가", 20),
	} {
		t.Run(tail[:3], func(t *testing.T) {
			prefix := "safe\n" + strings.Repeat("x", (64<<10)-len("safe\n")-2)
			var output boundedOutput
			for _, chunk := range []string{prefix, tail, "\nmore\n"} {
				n, err := io.WriteString(&output, chunk)
				if err != nil || n != len(chunk) {
					t.Fatal("discard must report the full write length")
				}
			}
			if !output.truncated || output.String() != "safe\n" {
				t.Errorf("incomplete line exposed (%d bytes)", len(output.String()))
			}
		})
	}
}

func TestBoundedOutputBoundaryAndUTF8(t *testing.T) {
	for _, size := range []int{(64 << 10) - 1, 64 << 10, (64 << 10) + 1} {
		var output boundedOutput
		value := strings.Repeat("x", size-1) + "\n"
		_, _ = io.WriteString(&output, value)
		if output.buffer.Len() > 64<<10 || output.truncated != (size > 64<<10) {
			t.Fatalf("boundary %d retained=%d truncated=%v", size, output.buffer.Len(), output.truncated)
		}
		if size <= 64<<10 && output.String() != value {
			t.Fatalf("boundary %d changed output", size)
		}
	}
	var output boundedOutput
	_, _ = io.WriteString(&output, "가\n"+strings.Repeat("나", 30000))
	if output.String() != "가\n" {
		t.Fatal("UTF-8 partial line was retained")
	}
}

func TestPolicyRunActualOutputAndTimeoutContracts(t *testing.T) {
	root := t.TempDir()
	for _, value := range []string{strings.Repeat("가", 15000), "token=fake-value\n", "safe\n" + strings.Repeat("x", (64<<10)-7) + "password=fake-value\n"} {
		if err := os.WriteFile(filepath.Join(root, "output.txt"), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
		result := runReadOnlyCommand(policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"cat", "output.txt"}, Timeout: "5s"})
		if !result.OK || result.ExitCode != 0 || !result.Executed || strings.Contains(result.Stdout, "fake-value") || !utf8.ValidString(result.Stdout) || len(result.Stdout) > 32*1024+len("\n<truncated>\n") {
			t.Fatalf("real output contract failed: exit=%d bytes=%d", result.ExitCode, len(result.Stdout))
		}
		if len(value) > 32*1024 && !strings.HasSuffix(result.Stdout, "\n<truncated>\n") {
			t.Error("real output lost truncation marker")
		}
	}
	result := runReadOnlyCommand(policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"awk", "BEGIN { while (1) {} }"}, Timeout: "100ms"})
	if result.OK || !result.Executed || !result.TimedOut || result.ExitCode != 124 || !strings.HasSuffix(result.Stderr, "command timed out\n") {
		t.Fatalf("real timeout contract: %+v", result)
	}
	result = runReadOnlyCommand(policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"false"}, Timeout: "5s"})
	if result.OK || !result.Executed || result.ExitCode != 1 || result.TimedOut {
		t.Fatalf("real nonzero contract: %+v", result)
	}
}

func TestCommandExecutorRejectsExpiredDeadlineBeforeStart(t *testing.T) {
	request := policycontract.CommandPolicyRequest{Argv: []string{filepath.Join(t.TempDir(), "missing")}}
	result := (CommandExecutor{}).Execute(request, time.Nanosecond)
	// Deadline wins before Start can return the missing executable's PathError.
	if !result.TimedOut || !errors.Is(result.Err, context.DeadlineExceeded) {
		t.Fatalf("expired start boundary: timeout=%v error=%v", result.TimedOut, result.Err)
	}
	root := t.TempDir()
	public := runReadOnlyCommand(policycontract.CommandPolicyRequest{WorkspaceRoot: root, CWD: root, Argv: []string{"true"}, Timeout: "1ns"})
	if public.OK || !public.Executed || !public.TimedOut || public.ExitCode != 124 {
		t.Fatalf("expired public timeout contract: %+v", public)
	}
}
