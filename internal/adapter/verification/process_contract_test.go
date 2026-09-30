package verification

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunCommandStepEnvWithBudgetCoversSuccessFailureAndOutputBudget(t *testing.T) {
	root := t.TempDir()
	helperTimeout := 15 * time.Second
	wrapperBin := writeCommandStepExecutable(t, root, "wrapper.sh", "#!/bin/sh\nprintf 'wrapper stdout'\n")
	wrapper := Run(root, "helper wrapper", helperTimeout, "", 32*1024, wrapperBin)
	if !wrapper.OK || wrapper.Label != "helper wrapper" || !strings.Contains(wrapper.Stdout, "wrapper stdout") {
		t.Fatalf("unexpected wrapper step: %+v", wrapper)
	}

	step := RunEnv(root, "helper success", helperTimeout, "stdin text", []string{"ISSUEOPS_HELPER_PROCESS=1", "ISSUEOPS_HELPER_VALUE=from-env"}, 32, os.Args[0], "-test.run=TestCommandStepHelperProcess", "--", "echo")
	if !step.OK || step.Label != "helper success" || !strings.Contains(step.Command, "-test.run=TestCommandStepHelperProcess") {
		t.Fatalf("unexpected success step: %+v", step)
	}
	if !step.StdoutTruncated || step.StdoutBytes == 0 || !strings.Contains(step.Stdout, "truncated") {
		t.Fatalf("expected truncated stdout metadata, got %+v", step)
	}
	if !strings.Contains(step.Stderr, "helper stderr") || step.StderrTruncated {
		t.Fatalf("unexpected stderr capture: %+v", step)
	}

	failed := RunEnv(root, "helper failure", helperTimeout, "", []string{"ISSUEOPS_HELPER_PROCESS=1"}, 32*1024, os.Args[0], "-test.run=TestCommandStepHelperProcess", "--", "fail")
	if failed.OK || !strings.Contains(failed.Error, "exit status 7") {
		t.Fatalf("unexpected failing step: %+v", failed)
	}
}

func TestCommandStepHelperProcess(t *testing.T) {
	if os.Getenv("ISSUEOPS_HELPER_PROCESS") != "1" {
		return
	}
	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
			break
		}
	}
	switch mode {
	case "echo":
		b, _ := io.ReadAll(os.Stdin)
		fmt.Fprintf(os.Stdout, "helper stdout %s %s %s", os.Getenv("ISSUEOPS_HELPER_VALUE"), string(b), strings.Repeat("x", 128))
		fmt.Fprint(os.Stderr, "helper stderr")
	case "fail":
		fmt.Fprint(os.Stderr, "helper failed")
		os.Exit(7)
	default:
		fmt.Fprint(os.Stderr, "unknown helper mode")
		os.Exit(64)
	}
	os.Exit(0)
}

func writeCommandStepExecutable(t *testing.T, root, name, content string) string {
	t.Helper()
	path := root + string(os.PathSeparator) + name
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
