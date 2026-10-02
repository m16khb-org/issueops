package projectdoc

import (
	"bytes"
	"errors"
	"io"
	projectdocdomain "issueops/internal/domain/projectdoc"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	projectDocCatalogMaxEntries   = 64
	projectDocCatalogDirBatch     = 128
	projectDocCatalogMaxFileBytes = 256 * 1024
	// projectDocCatalogHeaderBytes bounds the body read per document: only the
	// frontmatter description and the first H1 are used, so the first 8KiB is the
	// metadata window. 64 entries * 8KiB is the structural read ceiling.
	projectDocCatalogHeaderBytes = 8 * 1024
)

// projectdocdomain.ProjectDocCatalogEntry describes one project doc in the working repo's
// .issueops directory: its repo-relative path and a fixed description of
// what category of information it contains.
//
// The harness presents this catalog so the MAIN agent can decide which docs to
// read for the current prompt. The harness deliberately does not decide
// relevance itself: choosing "which docs matter for this prompt" is the
// judgment static analysis cannot do reliably, so it is left to the agent while
// the harness only provides an accurate, deterministic menu. The description is
// canonical metadata (from the doc's frontmatter or the name-keyed table), not a
// summary of the doc's current contents.

// DiscoverProjectDocs returns the catalog of <repoRoot>/.issueops/*.md for
// the repo currently being worked in. See DiscoverProjectDocsReport for the
// selection, read bounds, and omission accounting. Returns nil when repoRoot is
// empty or has no .issueops docs.
func DiscoverProjectDocs(repoRoot string) []projectdocdomain.ProjectDocCatalogEntry {
	entries, _, _ := DiscoverProjectDocsReport(repoRoot)
	return entries
}

// DiscoverProjectDocsReport lists the TARGET repo's <repoRoot>/.issueops/*.md
// project docs (not the harness's own documentation, not nested directories).
//
// The directory is scanned to EOF in batches of names, so the result no longer
// depends on filesystem order: the top 64 names by priority (standard required
// docs, then optional docs, then the rest lexically) are kept in a bounded
// sorted set, and the rest are counted as OverCap. Only the selected files are
// opened, each read is capped at the 8KiB metadata header, and files over the
// per-file size cap are skipped without reading any body bytes. Each doc's
// description comes from its meta frontmatter, falling back to the canonical
// name-keyed metadata. The entries are sorted by path so they can live in an
// immutable context prefix. Everything left out is counted in the returned
// omissions; stats expose the deterministic cost of the pass.
func DiscoverProjectDocsReport(repoRoot string) ([]projectdocdomain.ProjectDocCatalogEntry, projectdocdomain.CatalogOmissions, projectdocdomain.CatalogStats) {
	var omissions projectdocdomain.CatalogOmissions
	var stats projectdocdomain.CatalogStats
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return nil, omissions, stats
	}
	root, err := os.OpenRoot(repoRoot)
	if err != nil {
		return nil, omissions, stats
	}
	defer root.Close()
	docsInfo, err := root.Lstat(".issueops")
	if err != nil || !docsInfo.IsDir() || docsInfo.Mode()&os.ModeSymlink != 0 {
		return nil, omissions, stats
	}
	docsDir, err := root.Open(".issueops")
	if err != nil {
		omissions.ScanTruncated = true
		return nil, omissions, stats
	}
	defer docsDir.Close()

	selector, batches, scanTruncated := scanProjectDocNames(docsDir)
	stats.DirBatches = batches
	omissions.ScanTruncated = scanTruncated
	omissions.OverCap = selector.overCap

	catalog := []projectdocdomain.ProjectDocCatalogEntry{}
	for _, name := range selector.names() {
		header, truncated, status := readProjectDocCatalogHeader(root, filepath.Join(".issueops", name), &stats)
		switch status {
		case headerOversize:
			omissions.Oversize++
			continue
		case headerUnreadable:
			omissions.Unreadable++
			continue
		}
		entry, incomplete := buildProjectDocCatalogEntry(name, header, truncated)
		if incomplete {
			omissions.HeaderTruncated++
		}
		catalog = append(catalog, entry)
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].RelPath < catalog[j].RelPath })
	return catalog, omissions, stats
}

type projectDocDirReader interface {
	ReadDir(n int) ([]os.DirEntry, error)
}

// scanProjectDocNames reads the directory to EOF in name batches and feeds every
// regular, non-symlink .md name to a bounded selector. A read error other than
// EOF ends the scan with the partial selection and reports truncated=true.
func scanProjectDocNames(dir projectDocDirReader) (selector *projectDocNameSelector, batches int, truncated bool) {
	selector = newProjectDocNameSelector()
	for {
		entries, err := dir.ReadDir(projectDocCatalogDirBatch)
		if len(entries) > 0 {
			batches++
		}
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			selector.add(entry.Name())
		}
		if err != nil {
			return selector, batches, !errors.Is(err, io.EOF)
		}
		if len(entries) == 0 {
			return selector, batches, false
		}
	}
}

// projectDocNameSelector keeps the best projectDocCatalogMaxEntries names by
// (priority, name) in a sorted slice, so memory is bounded by the cap regardless
// of directory size and the outcome is independent of insertion order.
type projectDocNameSelector struct {
	priority map[string]int
	kept     []string
	overCap  int
}

func newProjectDocNameSelector() *projectDocNameSelector {
	priority := map[string]int{}
	for _, name := range projectdocdomain.OptionalProjectDocNames() {
		priority[name] = 1
	}
	for _, name := range projectdocdomain.ProjectDocNames() {
		priority[name] = 0
	}
	return &projectDocNameSelector{priority: priority, kept: make([]string, 0, projectDocCatalogMaxEntries)}
}

func (s *projectDocNameSelector) rank(name string) int {
	if rank, ok := s.priority[name]; ok {
		return rank
	}
	return 2
}

func (s *projectDocNameSelector) before(a, b string) bool {
	if ra, rb := s.rank(a), s.rank(b); ra != rb {
		return ra < rb
	}
	return a < b
}

func (s *projectDocNameSelector) add(name string) {
	pos := sort.Search(len(s.kept), func(i int) bool { return s.before(name, s.kept[i]) })
	if len(s.kept) == projectDocCatalogMaxEntries {
		s.overCap++
		if pos == len(s.kept) {
			return
		}
		s.kept = s.kept[:len(s.kept)-1]
	}
	s.kept = append(s.kept, "")
	copy(s.kept[pos+1:], s.kept[pos:])
	s.kept[pos] = name
}

func (s *projectDocNameSelector) names() []string {
	return s.kept
}

const (
	headerOK = iota
	headerOversize
	headerUnreadable
)

// readProjectDocCatalogHeader opens one selected doc under the root and returns
// at most its first 8KiB. A truncated header is cut after its last newline so a
// partial line is never interpreted. Oversize files cost no body bytes.
func readProjectDocCatalogHeader(root *os.Root, name string, stats *projectdocdomain.CatalogStats) (header string, truncated bool, status int) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return "", false, headerUnreadable
	}
	file, err := root.Open(name)
	if err != nil {
		return "", false, headerUnreadable
	}
	defer file.Close()
	stats.FilesOpened++
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() {
		return "", false, headerUnreadable
	}
	if opened.Size() > projectDocCatalogMaxFileBytes {
		return "", false, headerOversize
	}
	buf := make([]byte, projectDocCatalogHeaderBytes)
	n, err := io.ReadFull(file, buf)
	stats.BytesRead += n
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", false, headerUnreadable
	}
	data := buf[:n]
	truncated = n == projectDocCatalogHeaderBytes && opened.Size() > projectDocCatalogHeaderBytes
	if truncated {
		data = data[:bytes.LastIndexByte(data, '\n')+1]
	}
	return string(data), truncated, headerOK
}

// buildProjectDocCatalogEntry derives the entry from the header window. The
// second result reports a truncated window that did not hold the full metadata:
// an unclosed frontmatter block or no H1 inside the window.
func buildProjectDocCatalogEntry(name, header string, truncated bool) (projectdocdomain.ProjectDocCatalogEntry, bool) {
	_, description, body, ok := projectdocdomain.ParseFrontmatter(header)
	if !ok {
		body = header
	}
	if description == "" {
		if canonical, found := projectdocdomain.DocMetaDescription(name); found {
			description = canonical
		}
	}
	title := firstMarkdownHeading(body)
	unclosedFrontmatter := !ok && strings.TrimSpace(strings.SplitN(header, "\n", 2)[0]) == "---"
	return projectdocdomain.ProjectDocCatalogEntry{
		RelPath:     filepath.ToSlash(filepath.Join(".issueops", name)),
		Title:       title,
		Description: description,
	}, truncated && (title == "" || unclosedFrontmatter)
}

// FormatProjectDocCatalog renders a compact one-line menu of the project docs,
// describing each by its canonical metadata so the main agent can judge which to
// read. Returns "" when there is nothing to present.
func FormatProjectDocCatalog(entries []projectdocdomain.ProjectDocCatalogEntry) string {
	if len(entries) == 0 {
		return ""
	}
	items := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimPrefix(entry.RelPath, ".issueops/")
		meta := entry.Description
		if meta == "" {
			meta = entry.Title
		}
		if meta == "" {
			items = append(items, name)
			continue
		}
		items = append(items, name+"="+meta)
	}
	return "project docs (read what's relevant): " + strings.Join(items, "; ")
}

// FormatProjectDocCatalogOmissions returns the suffix appended to the compact
// menu with the exact omission counts, or "" when nothing was omitted.
func FormatProjectDocCatalogOmissions(omissions projectdocdomain.CatalogOmissions) string {
	if !omissions.Any() {
		return ""
	}
	return "; omitted: " + omissions.Summary()
}

// firstMarkdownHeading returns the first level-1 heading in a document body
// (frontmatter already stripped). Returns "" when there is no H1.
func firstMarkdownHeading(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:])
		}
	}
	return ""
}
