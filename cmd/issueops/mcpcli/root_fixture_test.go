package mcpcli

import (
	"os"
	"path/filepath"
)

const skillName = "atomic-commit-push"

const Version = "dev"

func IssueOpsRoot() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
		abs, err := filepath.Abs(root)
		if err == nil {
			return abs
		}
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "go.mod")) && fileExists(filepath.Join(dir, "skills")) {
			abs, err := filepath.Abs(dir)
			if err == nil {
				return abs
			}
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			abs, err := filepath.Abs(cwd)
			if err == nil {
				return abs
			}
			return cwd
		}
	}
}

func ResolveTarget(target string) string {
	if target != "" {
		return target
	}
	return IssueOpsRoot()
}

func ReadHarnessFile(parts ...string) (string, error) {
	path := filepath.Join(append([]string{IssueOpsRoot()}, parts...)...)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }
