package reviewfiles

import (
	"os"
	"path/filepath"
	"strings"
)

// ReadFile returns the content of file, cleaned and joined onto repo.
func ReadFile(repo, file string) (string, error) {
	data, err := os.ReadFile(filepath.Join(repo, filepath.Clean(file)))
	return string(data), err
}

// ReadResult reads file, resolving a relative path against repo, and returns the
// resolved path together with the bytes read.
func ReadResult(repo, file string) (string, []byte, error) {
	path := filepath.Clean(file)
	if !filepath.IsAbs(file) {
		path = filepath.Join(repo, file)
	}
	data, err := os.ReadFile(path)
	return path, data, err
}

// Mode returns the api_doc_mode value from the front matter of the repository's
// .issueops/OPEN_API_SPEC.md, or an empty string when absent or unreadable.
func Mode(repo string) string {
	content, err := os.ReadFile(filepath.Join(repo, ".issueops", "OPEN_API_SPEC.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		key, value, found := strings.Cut(trimmed, ":")
		if found && strings.TrimSpace(key) == "api_doc_mode" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
