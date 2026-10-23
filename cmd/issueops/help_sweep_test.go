package main

import (
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
		code, stdout := runCapturingStdout(t, append(path, "--help"))
		if code != 0 || strings.TrimSpace(stdout) == "" {
			t.Errorf("issueops %s --help: exit %d, stdout %d bytes", name, code, len(stdout))
		}
	}
}

func runCapturingStdout(t *testing.T, args []string) (int, string) {
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
	code := issueopsapp.RunRootCommand(args)
	os.Stdout, os.Stderr = stdout, stderr
	_ = write.Close()
	out := <-done
	_ = read.Close()
	return code, out
}
