// Package docs is the filesystem and git adapter behind the docs index: it lists
// Markdown candidates, tracked paths, document headings, and the documentation
// manifest's module directories for the docs application service.
package docs

import (
	"io/fs"
	"issueops/internal/adapter/outbound/gitbin"
	docscontract "issueops/internal/contract/docs"
	docsdomain "issueops/internal/domain/docs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Observer reads documentation candidates and metadata from disk and git.
type Observer struct{}

// Candidates returns the files under each of roots (relative to root): a file
// root is returned as is, and a directory root is walked for .md files while
// skipping excluded directories. Unreadable paths are skipped silently.
func (Observer) Candidates(root string, roots []string) []docscontract.Candidate {
	var candidates []docscontract.Candidate
	appendPath := func(path string) {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = ""
		}
		candidates = append(candidates, docscontract.Candidate{Path: path, RelPath: filepath.ToSlash(rel)})
	}
	for _, p := range roots {
		full := filepath.Join(root, p)
		info, err := os.Stat(full)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			appendPath(full)
			continue
		}
		_ = filepath.WalkDir(full, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if rel, relErr := filepath.Rel(root, path); relErr == nil && docsdomain.ExcludedDir(filepath.ToSlash(rel)) {
					return fs.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(path, ".md") {
				appendPath(path)
			}
			return nil
		})
	}
	return candidates
}

// TrackedPaths returns the repo's tracked paths relative to root
// (slash-separated), ok=true when git resolved a non-empty set. It uses -z
// (NUL-delimited, never C-quoted) and core.quotepath=false so non-ASCII (e.g.
// Korean) filenames match WalkDir's UTF-8 paths byte-for-byte, and never builds
// absolute paths (which would diverge from WalkDir under a symlinked root).
func (Observer) TrackedPaths(root string) (map[string]bool, bool) {
	out, err := exec.Command(gitbin.Path(), "-C", root, "-c", "core.quotepath=false", "ls-files", "-z").Output()
	if err != nil {
		return nil, false
	}
	set := make(map[string]bool)
	for p := range strings.SplitSeq(string(out), "\x00") {
		if p != "" {
			set[p] = true
		}
	}
	if len(set) == 0 {
		return nil, false
	}
	return set, true
}

// ReadDocument returns the index info (root-relative path, title, headings, byte
// size) for the file at path. It reports false when path is missing or a directory.
func (Observer) ReadDocument(root, path string) (docscontract.DocIndexInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return docscontract.DocIndexInfo{}, false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	title, headings := readHeadings(path)
	return docscontract.DocIndexInfo{RelPath: filepath.ToSlash(rel), Title: title, Headings: headings, Bytes: info.Size()}, true
}

// readHeadings returns the document title (the first level-1 heading, else
// the first heading) and up to 20 section headings after it; the title is not
// repeated in the list.
func readHeadings(path string) (string, []string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	title := ""
	headings := []string{}
	for line := range strings.SplitSeq(string(b), "\n") {
		level, text, ok := parseHeadingLine(strings.TrimSpace(line))
		if !ok {
			continue
		}
		if title == "" && level == 1 {
			title = text
			continue
		}
		headings = append(headings, text)
		if len(headings) > 20 {
			break
		}
	}
	if title == "" && len(headings) > 0 {
		title, headings = headings[0], headings[1:]
	}
	return title, headings[:min(len(headings), 20)]
}

// parseHeadingLine parses an ATX heading ("# " to "###### ") from a trimmed line.
func parseHeadingLine(line string) (int, string, bool) {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	text := strings.TrimSpace(line[level+1:])
	return level, text, text != ""
}
