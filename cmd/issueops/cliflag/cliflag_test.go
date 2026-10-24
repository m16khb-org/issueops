package cliflag

import (
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

// A mistyped flag is one returned error and no flag-list dump on either
// stream (that dump broke `--json 2>&1` parsing); --help lists the flags on
// stdout.
func TestParseIsQuietOnErrorsAndHelpGoesToStdout(t *testing.T) {
	newSet := func() *flag.FlagSet {
		fs := flag.NewFlagSet("demo", flag.ContinueOnError)
		fs.String("id", "", "cycle id")
		return fs
	}
	out, err := captureStdoutAndStderr(t, func() error { return Parse(newSet(), []string{"--bogus"}) })
	if err == nil || !strings.Contains(err.Error(), "-bogus") || out != "" {
		t.Fatalf("parse error: err=%v printed=%q", err, out)
	}
	out, err = captureStdoutAndStderr(t, func() error { return Parse(newSet(), []string{"--help"}) })
	if !errors.Is(err, flag.ErrHelp) || !strings.Contains(out, "-id") {
		t.Fatalf("help: err=%v printed=%q", err, out)
	}
}

func captureStdoutAndStderr(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = write, write
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(read)
		done <- string(data)
	}()
	runErr := fn()
	os.Stdout, os.Stderr = stdout, stderr
	_ = write.Close()
	return <-done, runErr
}

// An empty shell variable (`--id $ID --json`) must not turn the next flag into
// the value; anything that is not one of the command's flags still is a value.
func TestParseRejectsAValueFlagSwallowingAFlag(t *testing.T) {
	parse := func(args ...string) (string, bool, error) {
		fs := flag.NewFlagSet("demo", flag.ContinueOnError)
		id := fs.String("id", "", "cycle id")
		jsonOut := fs.Bool("json", false, "json")
		err := Parse(fs, args)
		return *id, *jsonOut, err
	}
	for _, args := range [][]string{{"--id", "--json"}, {"-id", "-json=true"}, {"--json", "--id", "--json"}} {
		if _, _, err := parse(args...); err == nil || !strings.Contains(err.Error(), "-id needs a value") {
			t.Fatalf("%q: err=%v", args, err)
		}
	}
	for _, tc := range []struct {
		args []string
		id   string
	}{
		{[]string{"--id", "--not-a-flag", "--json"}, "--not-a-flag"},
		{[]string{"--id=--json", "--json"}, "--json"},
		{[]string{"--id", "-", "--json"}, "-"},
		{[]string{"--json", "--", "--id", "--json"}, ""},
	} {
		id, jsonOut, err := parse(tc.args...)
		if err != nil || id != tc.id || !jsonOut {
			t.Fatalf("%q: id=%q json=%v err=%v", tc.args, id, jsonOut, err)
		}
	}
}

// A near-miss flag name names the flag the command has; an unrelated one
// does not guess.
func TestParseSuggestsTheClosestDefinedFlag(t *testing.T) {
	parse := func(arg string) error {
		fs := flag.NewFlagSet("demo", flag.ContinueOnError)
		fs.String("file", "", "ledger")
		fs.Bool("json", false, "json")
		return Parse(fs, []string{arg})
	}
	if err := parse("--files"); err == nil || !strings.HasSuffix(err.Error(), "(did you mean --file?)") {
		t.Fatalf("--files: %v", err)
	}
	if err := parse("--josn"); err == nil || !strings.HasSuffix(err.Error(), "(did you mean --json?)") {
		t.Fatalf("--josn: %v", err)
	}
	if err := parse("--zzzzzz"); err == nil || strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("--zzzzzz: %v", err)
	}
}

// zsh passes an unquoted flags variable as one argument; the error says so
// instead of suggesting the first flag name inside it.
func TestParseExplainsAnUnsplitFlagsVariable(t *testing.T) {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.String("host", "", "host")
	fs.String("session-id", "", "session")
	err := Parse(fs, []string{"--host claude --session-id s-1"})
	if err == nil || !strings.Contains(err.Error(), "one argument holds several flags") || !strings.Contains(err.Error(), "eval") {
		t.Fatalf("err=%v", err)
	}
}

// whoami's flags text is shell-quoted; zsh ${=VAR} splits it but keeps the
// quotes, so the session id arrived as 'abc' and the receipt failed later
// with no hint. Identity flags refuse literal quotes; other flags keep them.
func TestParseRefusesIdentityValuesStillCarryingQuotes(t *testing.T) {
	newFS := func() *flag.FlagSet {
		fs := flag.NewFlagSet("demo", flag.ContinueOnError)
		fs.String("session-id", "", "session")
		fs.String("evidence", "", "evidence")
		return fs
	}
	if err := Parse(newFS(), []string{"--session-id", "'abc'"}); err == nil || !strings.Contains(err.Error(), "--session-id value 'abc' still carries its quote characters") {
		t.Fatalf("quoted identity: %v", err)
	}
	if err := Parse(newFS(), []string{"--session-id", "abc", "--evidence", "'quoted'"}); err != nil {
		t.Fatalf("plain identity with quoted evidence: %v", err)
	}
}
