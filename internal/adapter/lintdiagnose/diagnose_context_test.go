package lintdiagnose

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/repopath"
	app "issueops/internal/application/lintdiagnose"
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
	"issueops/internal/testsupport"
)

// Cancelling the caller's context stops a command that does not finish on its
// own: Run kills it and returns context.Canceled within 5 s.
func TestRunReturnsWhenContextIsCancelled(t *testing.T) {
	dir := testsupport.WriteHangingCommand(t, "hang")
	argv := []string{filepath.Join(dir, testsupport.ExecutableName("hang"))}
	root := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, _, _, err := Effects{}.Run(ctx, root, argv)
		done <- err
	}()
	if !testsupport.WaitForFile(filepath.Join(dir, "started"), 10*time.Second) {
		t.Fatal("fake command did not start within 10s")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run error = %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return within 5s after the context was cancelled")
	}
}

// A command that outlives Service.CommandTimeout is killed at the bound, and
// Diagnose reports the timeout instead of diagnosing the partial output.
func TestDiagnoseTimesOutAtTheCommandBound(t *testing.T) {
	const bound = 200 * time.Millisecond
	dir := testsupport.WriteHangingCommand(t, "hang")
	service := app.Service{Effects: Effects{Normalize: repopath.NormalizeRoot}, CommandTimeout: bound}
	request := lintdiagnosecontract.Request{
		RepoRoot:    t.TempDir(),
		CommandArgv: []string{filepath.Join(dir, testsupport.ExecutableName("hang"))},
	}
	start := time.Now()
	result, err := service.Diagnose(t.Context(), request)
	elapsed := time.Since(start)
	if elapsed < bound || elapsed >= bound+runWaitDelay+2*time.Second {
		t.Fatalf("Diagnose returned after %s, want within [%s, %s)", elapsed, bound, bound+runWaitDelay+2*time.Second)
	}
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Diagnose error = %v, want a timed-out error wrapping context.DeadlineExceeded", err)
	}
	if result.Prompt != "" {
		t.Fatalf("Diagnose built a prompt from partial output: %q", result.Prompt)
	}
}

// A command that exits while a child it started still holds stdout keeps its
// own exit status: WaitDelay closes the pipe after the command exits, and Run
// reports the status with the output read so far and no error.
func TestRunKeepsTheExitStatusWhenAChildHoldsStdout(t *testing.T) {
	for _, tc := range []struct {
		name   string
		code   int
		failed bool
	}{
		{name: "exit 0", code: 0, failed: false},
		{name: "exit 3", code: 3, failed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := testsupport.WriteLingeringCommand(t, "lint", "lint ok", tc.code)
			argv := []string{filepath.Join(dir, testsupport.ExecutableName("lint"))}
			output, code, failed, err := Effects{}.Run(t.Context(), t.TempDir(), argv)
			if err != nil || code != tc.code || failed != tc.failed || !strings.Contains(output, "lint ok") {
				t.Fatalf("Run = %q, %d, %v, %v; want the printed output, %d, %v, nil", output, code, failed, err, tc.code, tc.failed)
			}
		})
	}
}
