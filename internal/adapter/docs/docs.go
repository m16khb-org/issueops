package docs

import (
	"io/fs"
	docscontract "issueops/internal/contract/docs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Observer struct{}

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
			if err == nil && !d.IsDir() && strings.HasSuffix(path, ".md") {
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
	out, err := exec.Command("git", "-C", root, "-c", "core.quotepath=false", "ls-files", "-z").Output()
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

func (Observer) ReadDocument(root, path string) (docscontract.DocIndexInfo, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return docscontract.DocIndexInfo{}, false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	title, headings := ReadHeadings(path)
	return docscontract.DocIndexInfo{RelPath: filepath.ToSlash(rel), Path: path, Title: title, Headings: headings, Bytes: info.Size()}, true
}

func ReadHeadings(path string) (string, []string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	title := ""
	headings := []string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "#") {
			continue
		}
		level := 0
		for level < len(line) && line[level] == '#' {
			level++
		}
		if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
			continue
		}
		text := strings.TrimSpace(line[level+1:])
		if text == "" {
			continue
		}
		if title == "" && level == 1 {
			title = text
		}
		headings = append(headings, text)
		if len(headings) >= 20 {
			break
		}
	}
	if title == "" && len(headings) > 0 {
		title = headings[0]
	}
	return title, headings
}
