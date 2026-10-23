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
