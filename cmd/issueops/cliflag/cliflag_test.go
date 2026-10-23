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
