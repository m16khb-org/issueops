package rootcmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"issueops/cmd/issueops/jsonout"
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
		{
			name:     "a hyphenated group and subcommand points at the group's help",
			args:     []string{"project-docs"},
			wantCode: 2,
			wantErr:  "Did you mean: project <subcommand> (issueops project --help lists them)",
		},
		{
			name:     "a word that is part of command paths lists those paths",
			args:     []string{"review"},
			wantCode: 2,
			wantErr:  "devils-advocate review, implementation-review record",
		},
		{
			name:     "a group subcommand typed alone names its group path",
			args:     []string{"sync-pr"},
			wantCode: 2,
			wantErr:  "Did you mean: remote sync-pr",
		},
		{
			name:     "unknown under --json answers one JSON document on stdout",
			args:     []string{"statu", "--json"},
			wantCode: 2,
			wantOut:  `"error": "unknown command \"statu\"; did you mean: status (issueops --help lists commands)"`,
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

// An unreported failure under --json is one {"ok":false,"error":...} document
// on stdout and nothing on stderr, so `--json 2>&1 | jq` parses it; without
// --json (or with --json only in passthrough argv) it stays a stderr line.
func TestUnreportedFailureUnderJSONIsOneEnvelopeOnStdout(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		wantJSON bool
	}{
		{[]string{"worker", "status", "--json"}, true},
		{[]string{"worker", "status", "--json=true"}, true},
		{[]string{"worker", "status"}, false},
		{[]string{"worker", "run", "--read-only", "--", "tool", "--json"}, false},
	} {
		var stdout, stderr bytes.Buffer
		cmd := testCommand(&stdout, &stderr)
		cmd.Runners["worker"] = func([]string) error { return errors.New("invalid worker job id") }
		if code := cmd.Run(tc.args); code != 1 {
			t.Fatalf("%q: exit code = %d, want 1", tc.args, code)
		}
		if !tc.wantJSON {
			if stdout.Len() != 0 || stderr.String() != "worker: invalid worker job id\n" {
				t.Fatalf("%q: stdout=%q stderr=%q, want the stderr line", tc.args, stdout.String(), stderr.String())
			}
			continue
		}
		var envelope struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || envelope.OK || envelope.Error != "worker: invalid worker job id" || stderr.Len() != 0 {
			t.Fatalf("%q: stdout=%q stderr=%q err=%v", tc.args, stdout.String(), stderr.String(), err)
		}
	}
}

// A command that printed its JSON result and returned an unmarked verdict
// error keeps stdout to that one document; the error goes to stderr.
func TestFailureAfterPrintedJSONResultAddsNoSecondDocument(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	realStdout := os.Stdout
	os.Stdout = write
	t.Cleanup(func() { os.Stdout = realStdout })
	var stdout, stderr bytes.Buffer
	cmd := testCommand(&stdout, &stderr)
	cmd.Runners["contract"] = func([]string) error {
		if err := jsonout.Print(map[string]any{"ok": false, "gate": "inconclusive"}); err != nil {
			return err
		}
		return errors.New("conformance_inconclusive")
	}
	code := cmd.Run([]string{"contract", "conformance", "live", "--json"})
	os.Stdout = realStdout
	_ = write.Close()
	printed, _ := io.ReadAll(read)
	if code != 1 || stdout.Len() != 0 || stderr.String() != "contract: conformance_inconclusive\n" {
		t.Fatalf("code=%d root stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(printed, &result); err != nil {
		t.Fatalf("command stdout is not one JSON document: %q", printed)
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
	cmd.Runners["project"] = func([]string) error { return nil }
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
