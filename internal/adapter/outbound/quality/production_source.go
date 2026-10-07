package quality

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const sourceGitTimeout = 10 * time.Second
const sourceGitOutputLimit = 64 * 1024 * 1024

// productionGoFiles preserves partial file results for the branch collector.
// SNR rejects any collection error instead of reporting an incomplete ratio.
func productionGoFiles(root string) ([]string, []error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, []error{err}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, []error{err}
	}
	if !info.IsDir() {
		return nil, []error{fmt.Errorf("quality source root is not a directory: %s", root)}
	}
	gitWorkspace, err := hasGitWorkspace(root)
	if err != nil {
		return nil, []error{err}
	}
	var candidates []string
	var scanErrors []error
	if gitWorkspace != "" {
		ctx, cancel := context.WithTimeout(context.Background(), sourceGitTimeout)
		defer cancel()
		output, err := sourceGitOutput(ctx, root, sourceGitOutputLimit, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
		if err != nil {
			return nil, []error{err}
		}
		scope, err := filepath.Rel(gitWorkspace, root)
		if err != nil {
			return nil, []error{err}
		}
		if excludedSourcePath(scope) {
			return []string{}, nil
		}
		candidates = strings.Split(output, "\x00")
	} else {
		if excludedSourcePath(root) {
			return []string{}, nil
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				scanErrors = append(scanErrors, err)
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			if excludedSourcePath(relative) && entry.IsDir() {
				return filepath.SkipDir
			}
			if !entry.IsDir() {
				candidates = append(candidates, relative)
			}
			return nil
		})
		if err != nil {
			scanErrors = append(scanErrors, err)
		}
	}
	files := []string{}
	seen := map[string]bool{}
	for _, relative := range candidates {
		if relative == "" {
			continue
		}
		relative = filepath.Clean(relative)
		if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			scanErrors = append(scanErrors, fmt.Errorf("quality source path escapes root: %s", relative))
			continue
		}
		if excludedSourcePath(relative) || !strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, "_test.go") || seen[relative] {
			continue
		}
		seen[relative] = true
		path := filepath.Join(root, relative)
		regular, err := regularSourcePath(root, relative)
		if err != nil {
			scanErrors = append(scanErrors, err)
			continue
		}
		if !regular {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			scanErrors = append(scanErrors, err)
			continue
		}
		// PackageClauseOnly reads the generated header without changing the existing
		// collectors' handling of malformed function bodies or SNR line limits.
		file, _ := parser.ParseFile(token.NewFileSet(), path, data, parser.PackageClauseOnly|parser.ParseComments)
		if file != nil && ast.IsGenerated(file) {
			continue
		}
		files = append(files, path)
	}
	sort.Strings(files)
	return files, scanErrors
}

func excludedSourcePath(relative string) bool {
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		switch component {
		case ".git", ".codegraph", ".issueops-runtime", ".issueops", "bin", "vendor", "node_modules", "testdata":
			return true
		}
	}
	return false
}

// A worktree .git file is also a marker. Missing or broken Git in a marked
// workspace is an error; only ordinary directories use filesystem discovery.
func hasGitWorkspace(root string) (string, error) {
	for directory := root; ; directory = filepath.Dir(directory) {
		_, err := os.Lstat(filepath.Join(directory, ".git"))
		if err == nil {
			return directory, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if filepath.Dir(directory) == directory {
			return "", nil
		}
	}
}

func regularSourcePath(root, relative string) (bool, error) {
	path := root
	components := strings.Split(relative, string(filepath.Separator))
	for index, component := range components {
		path = filepath.Join(path, component)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		} // Deleted tracked files are absent.
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, nil
		}
		if index == len(components)-1 {
			return info.Mode().IsRegular(), nil
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	return false, nil
}

func sourceGitOutput(ctx context.Context, root string, limit int, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = root
	stdout := NewBoundedQualityBuffer(limit)
	stderr := NewBoundedQualityBuffer(64 * 1024)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("quality source Git: %w", ctx.Err())
	}
	if stdout.Truncated() || stderr.Truncated() {
		return "", fmt.Errorf("quality source Git output exceeds limit")
	}
	if err != nil {
		return "", fmt.Errorf("quality source Git: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
