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

// ListUnformatted runs gofmt -l on files in root and returns the ones gofmt
// would rewrite. A gofmt failure is returned with its stderr text attached.
func ListUnformatted(ctx context.Context, root string, files []string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gofmt", append([]string{"-l"}, files...)...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, commandError(err, stderr.String())
	}
	return nonEmptyLines(string(out)), nil
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
