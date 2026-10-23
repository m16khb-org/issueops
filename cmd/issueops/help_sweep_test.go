package main

import (
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"

	issueopsapp "issueops/cmd/issueops/issueopsapp"
	clicatalog "issueops/internal/adapter/inbound/catalog/cli"
)

// Every command path the usage lists answers `--help` with exit 0 and text on
// stdout, so `issueops <cmd> --help | grep` works and an agent probing a
// subcommand is not told it failed.
func TestEveryListedCommandPathAnswersHelpOnStdout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Chdir(t.TempDir())
	word := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	paths := map[string][]string{}
	for line := range strings.SplitSeq(clicatalog.Usage("test"), "\n") {
		fields := strings.Fields(line)
		if !strings.HasPrefix(line, "  issueops ") || len(fields) < 2 {
			continue
		}
		var words []string
		for _, field := range fields[1:] {
			if !word.MatchString(field) || len(words) == 3 {
				break
			}
			words = append(words, field)
			paths[strings.Join(words, " ")] = append([]string(nil), words...)
		}
	}
	if len(paths) < 100 {
		t.Fatalf("usage yielded only %d command paths", len(paths))
	}
	for name, path := range paths {
		code, stdout, _ := runCapturing(t, append(path, "--help"))
		if code != 0 || strings.TrimSpace(stdout) == "" {
			t.Errorf("issueops %s --help: exit %d, stdout %d bytes", name, code, len(stdout))
		}
	}
	// A command group given a subcommand it lacks fails with one JSON
	// document under --json and nothing on stderr, so `2>&1 | json.load`
	// still parses.
	for name, path := range paths {
		if !isGroup(paths, name) {
			continue
		}
		code, stdout, stderr := runCapturing(t, append(path, "no-such-subcommand", "--json"))
		var doc map[string]any
		decoder := json.NewDecoder(strings.NewReader(stdout))
		if code == 0 || decoder.Decode(&doc) != nil || decoder.More() || doc["error"] == nil || strings.TrimSpace(stderr) != "" {
			t.Errorf("issueops %s no-such-subcommand --json: exit %d, stdout %q, stderr %q", name, code, stdout, stderr)
		}
	}
}

func isGroup(paths map[string][]string, name string) bool {
	for other := range paths {
		if strings.HasPrefix(other, name+" ") {
			return true
		}
	}
	return false
}

func runCapturing(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	capture := func() (*os.File, <-chan string) {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan string, 1)
		go func() {
			data, _ := io.ReadAll(read)
			_ = read.Close()
			done <- string(data)
		}()
		return write, done
	}
	outWrite, outDone := capture()
	errWrite, errDone := capture()
	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outWrite, errWrite
	code := issueopsapp.RunRootCommand(args)
	os.Stdout, os.Stderr = stdout, stderr
	_ = outWrite.Close()
	_ = errWrite.Close()
	return code, <-outDone, <-errDone
}
