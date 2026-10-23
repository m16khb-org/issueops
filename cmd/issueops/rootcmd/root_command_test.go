package rootcmd

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestRunRootCommandUsageAndVersionSurface(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
		wantErr  string
	}{
		{
			name:     "no args prints usage to stderr",
			args:     []string{},
			wantCode: 2,
			wantErr:  "issueops 0.1.0",
		},
		{
			name:     "help prints usage to stderr",
			args:     []string{"help"},
			wantCode: 0,
			wantErr:  "issueops 0.1.0",
		},
		{
			name:     "version prints stdout",
			args:     []string{"version"},
			wantCode: 0,
			wantOut:  "issueops 0.1.0\n",
		},
		{
			name:     "unknown prints a short error with suggestions",
			args:     []string{"statu"},
			wantCode: 2,
			wantErr:  "issueops: unknown command \"statu\"\nDid you mean: status\nRun 'issueops --help' for the full usage.\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, errText, code := captureRootCommandOutput(tc.args)
			if code != tc.wantCode {
				t.Fatalf("exit code = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(out, tc.wantOut) {
				t.Fatalf("stdout = %q, want containing %q", out, tc.wantOut)
			}
			if !strings.Contains(errText, tc.wantErr) {
				t.Fatalf("stderr = %q, want containing %q", errText, tc.wantErr)
			}
		})
	}
}

func TestRunRootCommandDispatchesRunner(t *testing.T) {
	var gotArgs []string
	cmd := testCommand(&bytes.Buffer{}, &bytes.Buffer{})
	cmd.Runners["status"] = func(args []string) error {
		gotArgs = append([]string{}, args...)
		return nil
	}

	if code := cmd.Run([]string{"status", "--json"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "--json" {
		t.Fatalf("runner args = %#v, want --json", gotArgs)
	}
}

func TestRunRootCommandUsesCustomErrorExitCode(t *testing.T) {
	var stderr bytes.Buffer
	cmd := testCommand(&bytes.Buffer{}, &stderr)
	cmd.Runners["guard"] = func([]string) error {
		return errors.New("blocked")
	}
	cmd.ErrorExitCode = func(name string, err error) int {
		if name == "guard" && err != nil {
			return 3
		}
		return 1
	}

	if code := cmd.Run([]string{"guard"}); code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if !strings.Contains(stderr.String(), "guard: blocked") {
		t.Fatalf("stderr = %q, want guard error", stderr.String())
	}
}

// A subcommand answering --help returns flag.ErrHelp after printing its own
// usage. That is a successful request: exit 0 and no "name: flag: help
// requested" line on stderr.
func TestRunRootCommandTreatsHelpRequestAsSuccess(t *testing.T) {
	var stderr bytes.Buffer
	cmd := testCommand(&bytes.Buffer{}, &stderr)
	cmd.Runners["hook"] = func([]string) error { return flag.ErrHelp }

	if code := cmd.Run([]string{"hook", "--help"}); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want nothing for a help request", stderr.String())
	}
}

func TestHelpRequestedStopsAtPassthroughSeparator(t *testing.T) {
	cases := map[string]struct {
		args []string
		want bool
	}{
		"leading help":           {[]string{"help"}, true},
		"subcommand help flag":   {[]string{"decision", "add", "--help"}, true},
		"single dash":            {[]string{"gates", "init", "-h"}, true},
		"help as a flag value":   {[]string{"state", "write", "--value", "help"}, false},
		"help after passthrough": {[]string{"policy", "check", "--", "git", "--help"}, false},
		"no help":                {[]string{"status", "--json"}, false},
	}
	for name, tc := range cases {
		if got := HelpRequested(tc.args); got != tc.want {
			t.Errorf("%s: HelpRequested(%q) = %v, want %v", name, tc.args, got, tc.want)
		}
	}
}

func TestUnknownCommandSuggestsClosestFirst(t *testing.T) {
	var stderr bytes.Buffer
	cmd := testCommand(&bytes.Buffer{}, &stderr)
	for _, name := range []string{"status", "state", "start", "inspect", "next"} {
		cmd.Runners[name] = func([]string) error { return nil }
	}
	if code := cmd.Run([]string{"statu"}); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	// "status" extends the typo, "state" is one edit away, "start" two; "inspect" and "next" are unrelated.
	if !strings.Contains(stderr.String(), "Did you mean: status, state, start\n") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func captureRootCommandOutput(args []string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	cmd := testCommand(&stdout, &stderr)
	cmd.Runners["status"] = func([]string) error { return nil }
	code := cmd.Run(args)
	return stdout.String(), stderr.String(), code
}

func testCommand(stdout, stderr *bytes.Buffer) Command {
	return Command{
		Version: "0.1.0",
		Usage: func() {
			stderr.WriteString("issueops 0.1.0\n")
		},
		Stdout:  stdout,
		Stderr:  stderr,
		Runners: map[string]Runner{},
		ErrorExitCode: func(string, error) int {
			return 1
		},
	}
}
