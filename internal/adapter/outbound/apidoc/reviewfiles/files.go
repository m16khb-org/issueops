package reviewfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	domain "issueops/internal/domain/apidoc"
)

func ExtraPrompt(repo, promptFile string) (string, error) {
	if promptFile != "" {
		b, err := os.ReadFile(promptFile)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if b, err := os.ReadFile(filepath.Join(repo, ".issueops", "OPEN_API_SPEC.md")); err == nil {
		return string(b), nil
	}
	return "", nil
}

func (f Files) Diff(repo string, files []string, diffFile string) (string, error) {
	if diffFile != "" {
		b, err := os.ReadFile(diffFile)
		return string(b), err
	}
	args := append([]string{"diff", "--cached", "--"}, files...)
	code, out, stderr := f.GitCmd(repo, args...)
	if code != 0 {
		return "", fmt.Errorf("git diff failed: %s", stderr)
	}
	return out, nil
}

func (f Files) Input(repo string, files []string, diffFile string, all bool) (string, error) {
	if all && diffFile == "" {
		return FullContent(repo, files)
	}
	return f.Diff(repo, files, diffFile)
}

func FullContent(repo string, files []string) (string, error) {
	var b strings.Builder
	for _, file := range files {
		clean := filepath.Clean(file)
		if filepath.IsAbs(clean) || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
			return "", fmt.Errorf("unsafe file path %q", file)
		}
		content, err := os.ReadFile(filepath.Join(repo, clean))
		if err != nil {
			return "", err
		}
		b.WriteString("\n--- FILE: ")
		b.WriteString(clean)
		b.WriteString(" ---\n")
		b.Write(content)
		if len(content) == 0 || content[len(content)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

func (f Files) Staged(repo string) []string {
	code, out, _ := f.GitCmd(repo, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "--")
	if code != 0 {
		return nil
	}
	return Normalize(repo, splitLines(out))
}

func (f Files) Tracked(repo string) []string {
	code, out, _ := f.GitCmd(repo, "ls-files")
	if code != 0 {
		return nil
	}
	var files []string
	for _, line := range strings.Split(out, "\n") {
		file := strings.TrimSpace(line)
		if file == "" || !domain.IsCandidate(file) {
			continue
		}
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func Normalize(repo string, files []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, file := range files {
		file = strings.TrimSpace(file)
		if file == "" || !domain.IsCandidate(file) {
			continue
		}
		if filepath.IsAbs(file) {
			if rel, err := filepath.Rel(repo, file); err == nil {
				file = rel
			}
		}
		file = filepath.ToSlash(filepath.Clean(file))
		if file == "." || strings.HasPrefix(file, "../") || seen[file] {
			continue
		}
		seen[file] = true
		out = append(out, file)
	}
	sort.Strings(out)
	return out
}

func splitLines(text string) []string {
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
