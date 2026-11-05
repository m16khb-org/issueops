// Package pathutil resolves where the issueops harness and the target repository
// live on disk: the harness root, marker-based upward searches, and the target
// directory for CLI and hook commands.
package pathutil

import (
	"os"
	"path/filepath"
)

// ReadHarnessFile reads the file at root joined with parts and returns its
// contents as a string. Read errors are returned unchanged.
func ReadHarnessFile(root string, parts ...string) (string, error) {
	path := filepath.Join(append([]string{root}, parts...)...)
	b, err := os.ReadFile(path)
	return string(b), err
}

// IssueOpsRoot locates the harness root. It uses ISSUEOPS_ROOT when set, otherwise
// the first directory containing marker found by searching upward from the working
// directory and the executable (including its symlink-resolved location), and
// finally the working directory itself.
func IssueOpsRoot(marker string) string {
	envRoot := os.Getenv("ISSUEOPS_ROOT")
	cwd, _ := os.Getwd()
	executable, _ := os.Executable()
	return issueOpsRootFrom(marker, envRoot, cwd, executable)
}

func issueOpsRootFrom(marker, envRoot, cwd, executable string) string {
	if envRoot != "" {
		if root, err := filepath.Abs(envRoot); err == nil {
			return root
		}
	}
	var starts []string
	if cwd != "" {
		starts = append(starts, cwd)
	}
	if executable != "" {
		executableDir := filepath.Dir(executable)
		starts = append(starts, executableDir, filepath.Dir(executableDir))
		if resolved, err := filepath.EvalSymlinks(executable); err == nil && resolved != executable {
			resolvedDir := filepath.Dir(resolved)
			starts = append(starts, resolvedDir, filepath.Dir(resolvedDir))
		}
	}
	for _, start := range starts {
		if root, ok := FindUp(start, marker); ok {
			return root
		}
	}
	if cwd != "" {
		return cwd
	}
	return "."
}

// FindUp walks from start toward the filesystem root and returns the first
// directory that contains marker. It reports false when none does or start
// cannot be made absolute.
func FindUp(start, marker string) (string, bool) {
	d, err := filepath.Abs(start)
	if err != nil {
		return "", false
	}
	for {
		if exists(filepath.Join(d, marker)) {
			return d, true
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

// ResolveTarget returns the absolute target directory for arg. An empty arg falls
// back to CLAUDE_PROJECT_DIR, then PWD, then the working directory, then ".".
func ResolveTarget(arg string) string {
	if arg == "" {
		if env := os.Getenv("CLAUDE_PROJECT_DIR"); env != "" {
			arg = env
		} else if env := os.Getenv("PWD"); env != "" {
			arg = env
		} else if cwd, err := os.Getwd(); err == nil {
			arg = cwd
		} else {
			arg = "."
		}
	}
	abs, err := filepath.Abs(arg)
	if err != nil {
		return arg
	}
	return abs
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
