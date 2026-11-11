package verification

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/outbound/gitbin"
)

// ListTrackedGoFiles returns the git-tracked .go files under root as
// root-relative paths. Files that git still lists but that no longer exist on
// disk are skipped.
func ListTrackedGoFiles(ctx context.Context, root string) ([]string, error) {
	cmd := exec.CommandContext(ctx, gitbin.Path(), "-C", root, "ls-files", "-z", "--", "*.go")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, commandError(err, stderr.String())
	}
	files := []string{}
	for file := range strings.SplitSeq(string(out), "\x00") {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, file)); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// gofmtArgBudget caps the bytes of file arguments in one gofmt call. Windows
// limits a command line to 32,767 characters, so a single call over every
// tracked file fails there; the same batches run on every OS.
const gofmtArgBudget = 24000

// ListUnformatted runs gofmt -l on files in root and returns the ones gofmt
// would rewrite, in input order. The files go to gofmt in batches that fit
// gofmtArgBudget. It stops at the first failing batch and returns that batch's
// error with its stderr text attached.
func ListUnformatted(ctx context.Context, root string, files []string) ([]string, error) {
	return listUnformatted(ctx, root, files, gofmtArgBudget)
}

func listUnformatted(ctx context.Context, root string, files []string, budget int) ([]string, error) {
	unformatted := []string{}
	for _, batch := range gofmtBatches(files, budget) {
		cmd := exec.CommandContext(ctx, "gofmt", append([]string{"-l"}, batch...)...)
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, commandError(err, stderr.String())
		}
		unformatted = append(unformatted, nonEmptyLines(string(out))...)
	}
	return unformatted, nil
}

// gofmtBatches splits files, keeping their order, into batches whose paths
// (each counted with one separator byte) stay within budget. A path longer
// than budget gets a batch of its own.
func gofmtBatches(files []string, budget int) [][]string {
	var batches [][]string
	var current []string
	used := 0
	for _, file := range files {
		size := len(file) + 1
		if len(current) > 0 && used+size > budget {
			batches = append(batches, current)
			current, used = nil, 0
		}
		current = append(current, file)
		used += size
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func commandError(err error, stderr string) error {
	if detail := strings.TrimSpace(stderr); detail != "" {
		return fmt.Errorf("%w: %s", err, detail)
	}
	return err
}

func nonEmptyLines(text string) []string {
	lines := []string{}
	for line := range strings.SplitSeq(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
