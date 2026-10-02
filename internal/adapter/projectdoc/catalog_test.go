package projectdoc

import (
	"errors"
	"fmt"
	projectdocdomain "issueops/internal/domain/projectdoc"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverProjectDocsUsesFrontmatterThenCanonicalMeta(t *testing.T) {
	repo := t.TempDir()
	writeProjectDoc(t, repo, "CUSTOM.md", "---\nname: CUSTOM.md\ndescription: 이 레포 전용 메모를 담는다.\n---\n\n# 커스텀\n")
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n\n## 경계\n")
	if err := os.MkdirAll(filepath.Join(repo, ".issueops", "state"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectDoc(t, repo, "notes.txt", "ignored")

	catalog := DiscoverProjectDocs(repo)
	if len(catalog) != 2 {
		t.Fatalf("expected 2 project docs, got %d: %+v", len(catalog), catalog)
	}
	if catalog[0].RelPath != ".issueops/ARCHITECTURE.md" || catalog[1].RelPath != ".issueops/CUSTOM.md" {
		t.Fatalf("catalog not sorted by rel path: %+v", catalog)
	}
	canonical, _ := projectdocdomain.DocMetaDescription("ARCHITECTURE.md")
	if catalog[0].Description != canonical {
		t.Fatalf("expected canonical meta fallback, got %q", catalog[0].Description)
	}
	if catalog[1].Description != "이 레포 전용 메모를 담는다." {
		t.Fatalf("expected frontmatter description, got %q", catalog[1].Description)
	}
	if catalog[0].Title != "아키텍처" {
		t.Fatalf("expected H1 title, got %q", catalog[0].Title)
	}
}

func TestDiscoverProjectDocsEmptyWhenNoAgentHarness(t *testing.T) {
	if got := DiscoverProjectDocs(t.TempDir()); got != nil {
		t.Fatalf("expected nil catalog for repo without .issueops, got %+v", got)
	}
	if got := DiscoverProjectDocs(""); got != nil {
		t.Fatalf("expected nil catalog for empty repo root, got %+v", got)
	}
}

func TestDiscoverProjectDocsSkipsSymlinkAndNonRegularMarkdown(t *testing.T) {
	repo := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("# Outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(repo, ".issueops")
	if err := os.MkdirAll(filepath.Join(dir, "directory.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "outside.md")); err != nil {
		t.Fatal(err)
	}
	writeProjectDoc(t, repo, "inside.md", "# Inside\n")

	catalog := DiscoverProjectDocs(repo)
	if len(catalog) != 1 || catalog[0].RelPath != ".issueops/inside.md" {
		t.Fatalf("catalog must exclude symlinked and non-regular markdown: %+v", catalog)
	}
}

func TestDiscoverProjectDocsRejectsSymlinkedAgentHarnessDirectory(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.md"), []byte("# Outside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, ".issueops")); err != nil {
		t.Fatal(err)
	}

	if got := DiscoverProjectDocs(repo); got != nil {
		t.Fatalf("symlinked .issueops directory must be ignored: %+v", got)
	}
}

func TestDiscoverProjectDocsBoundsEntriesAndContent(t *testing.T) {
	repo := t.TempDir()
	for index := 0; index < projectDocCatalogMaxEntries+1; index++ {
		writeProjectDoc(t, repo, fmt.Sprintf("entry-%02d.md", index), "# Entry\n")
	}
	if got := DiscoverProjectDocs(repo); len(got) != projectDocCatalogMaxEntries {
		t.Fatalf("catalog entries = %d, want %d", len(got), projectDocCatalogMaxEntries)
	}

	perFileRepo := t.TempDir()
	writeProjectDoc(t, perFileRepo, "oversize.md", strings.Repeat("x", projectDocCatalogMaxFileBytes+1))
	if got := DiscoverProjectDocs(perFileRepo); len(got) != 0 {
		t.Fatalf("oversize document must be skipped: %+v", got)
	}

}

func TestDiscoverReadsBoundedHeader(t *testing.T) {
	repo := t.TempDir()
	const files = 9
	for index := 0; index < files; index++ {
		writeProjectDoc(t, repo, fmt.Sprintf("entry-%02d.md", index), "# Entry\n"+strings.Repeat("x", 240*1024))
	}

	catalog, omissions, stats := DiscoverProjectDocsReport(repo)
	if len(catalog) != files {
		t.Fatalf("catalog entries = %d, want %d", len(catalog), files)
	}
	for _, entry := range catalog {
		if entry.Title != "Entry" {
			t.Fatalf("title lost with bounded header: %+v", entry)
		}
	}
	if stats.FilesOpened != files || stats.BytesRead > files*projectDocCatalogHeaderBytes {
		t.Fatalf("stats = %+v, want %d files and <= %d bytes", stats, files, files*projectDocCatalogHeaderBytes)
	}
	if omissions != (projectdocdomain.CatalogOmissions{}) {
		t.Fatalf("nothing was omitted, got %+v", omissions)
	}
}

func TestDiscoverCapsEntriesAndCountsOverCap(t *testing.T) {
	repo := t.TempDir()
	total := projectDocCatalogMaxEntries + 6
	for index := 0; index < total; index++ {
		writeProjectDoc(t, repo, fmt.Sprintf("entry-%03d.md", index), "# Entry\n")
	}

	catalog, omissions, _ := DiscoverProjectDocsReport(repo)
	if len(catalog) != projectDocCatalogMaxEntries || omissions.OverCap != 6 {
		t.Fatalf("entries=%d over_cap=%d, want %d and 6", len(catalog), omissions.OverCap, projectDocCatalogMaxEntries)
	}
	if catalog[0].RelPath != ".issueops/entry-000.md" || catalog[len(catalog)-1].RelPath != ".issueops/entry-063.md" {
		t.Fatalf("selection must be the lexical prefix: first=%s last=%s", catalog[0].RelPath, catalog[len(catalog)-1].RelPath)
	}
}

func TestDiscoverSelectionIndependentOfCreationOrder(t *testing.T) {
	for _, total := range []int{200, 1200} {
		selected := make([][]string, 2)
		var overCaps [2]int
		var batches [2]int
		for run, descending := range []bool{false, true} {
			repo := t.TempDir()
			for index := 0; index < total; index++ {
				n := index
				if descending {
					n = total - 1 - index
				}
				writeProjectDoc(t, repo, fmt.Sprintf("doc-%04d.md", n), "# Doc\n")
			}
			catalog, omissions, stats := DiscoverProjectDocsReport(repo)
			for _, entry := range catalog {
				selected[run] = append(selected[run], entry.RelPath)
			}
			overCaps[run], batches[run] = omissions.OverCap, stats.DirBatches
		}
		if strings.Join(selected[0], ",") != strings.Join(selected[1], ",") {
			t.Fatalf("%d docs: selection depends on creation order", total)
		}
		wantBatches := (total + projectDocCatalogDirBatch - 1) / projectDocCatalogDirBatch
		if len(selected[0]) != projectDocCatalogMaxEntries || overCaps[0] != total-projectDocCatalogMaxEntries || overCaps[1] != overCaps[0] || batches[0] != wantBatches {
			t.Fatalf("%d docs: selected=%d over_cap=%v batches=%v want batches=%d", total, len(selected[0]), overCaps, batches, wantBatches)
		}
		if selected[0][0] != ".issueops/doc-0000.md" || selected[0][len(selected[0])-1] != ".issueops/doc-0063.md" {
			t.Fatalf("%d docs: not the lexical prefix: %s .. %s", total, selected[0][0], selected[0][len(selected[0])-1])
		}
	}
}

func TestProjectDocNameSelectorIgnoresInputOrder(t *testing.T) {
	names := []string{"ARCHITECTURE.md", "VCS.md", "ADR.md"}
	for index := 0; index < 100; index++ {
		names = append(names, fmt.Sprintf("0-%03d.md", index))
	}
	orders := [][]string{append([]string(nil), names...)}
	reversed := make([]string, len(names))
	for index, name := range names {
		reversed[len(names)-1-index] = name
	}
	rotated := append(append([]string(nil), names[37:]...), names[:37]...)
	orders = append(orders, reversed, rotated)

	var want string
	for run, order := range orders {
		selector := newProjectDocNameSelector()
		for _, name := range order {
			selector.add(name)
		}
		got := strings.Join(selector.names(), ",")
		if run == 0 {
			want = got
		}
		if got != want || selector.overCap != len(names)-projectDocCatalogMaxEntries {
			t.Fatalf("order %d: selection differs or over_cap=%d", run, selector.overCap)
		}
	}
	if !strings.HasPrefix(want, "ADR.md,ARCHITECTURE.md,VCS.md,0-000.md,") {
		t.Fatalf("required, then optional, then others lexically: %.60s", want)
	}
}

func TestDiscoverRequiredDocsSurviveCap(t *testing.T) {
	repo := t.TempDir()
	for index := 0; index < projectDocCatalogMaxEntries+6; index++ {
		writeProjectDoc(t, repo, fmt.Sprintf("0-%02d.md", index), "# Filler\n")
	}
	writeProjectDoc(t, repo, "ARCHITECTURE.md", "# 아키텍처\n")
	writeProjectDoc(t, repo, "VCS.md", "# VCS\n")

	catalog, omissions, _ := DiscoverProjectDocsReport(repo)
	found := map[string]bool{}
	for _, entry := range catalog {
		found[entry.RelPath] = true
	}
	if !found[".issueops/ARCHITECTURE.md"] || !found[".issueops/VCS.md"] {
		t.Fatalf("required/optional docs must survive the cap: %+v", catalog)
	}
	if len(catalog) != projectDocCatalogMaxEntries || omissions.OverCap != 8 {
		t.Fatalf("entries=%d over_cap=%d, want %d and 8", len(catalog), omissions.OverCap, projectDocCatalogMaxEntries)
	}
}

func TestDiscoverReportsOversizeAndUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission denial cannot be produced as root")
	}
	repo := t.TempDir()
	writeProjectDoc(t, repo, "a-oversize.md", strings.Repeat("x", projectDocCatalogMaxFileBytes+1))
	writeProjectDoc(t, repo, "a-unreadable.md", "# Hidden\n")
	if err := os.Chmod(filepath.Join(repo, ".issueops", "a-unreadable.md"), 0); err != nil {
		t.Fatal(err)
	}
	writeProjectDoc(t, repo, "b-ok.md", "# Ok\n")

	catalog, omissions, stats := DiscoverProjectDocsReport(repo)
	if len(catalog) != 1 || catalog[0].RelPath != ".issueops/b-ok.md" {
		t.Fatalf("only the readable document is cataloged: %+v", catalog)
	}
	want := projectdocdomain.CatalogOmissions{Oversize: 1, Unreadable: 1}
	if omissions != want {
		t.Fatalf("omissions = %+v, want %+v", omissions, want)
	}
	if stats.BytesRead > projectDocCatalogHeaderBytes {
		t.Fatalf("oversize and unreadable files must cost no body bytes: %+v", stats)
	}
}

func TestDiscoverReportsUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission denial cannot be produced as root")
	}
	repo := t.TempDir()
	writeProjectDoc(t, repo, "NOTE.md", "# Hidden\n")
	dir := filepath.Join(repo, ".issueops")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Error(err)
		}
	})

	_, omissions, _ := DiscoverProjectDocsReport(repo)
	if !omissions.ScanTruncated {
		t.Fatal("an unreadable document directory was reported as an empty complete scan")
	}
}

func TestDiscoverHeaderTruncationRules(t *testing.T) {
	pad := func(lines int) string { return strings.Repeat("a\n", lines) }
	cases := []struct {
		name      string
		content   string
		wantTitle string
		wantDesc  string
		wantTrunc int
	}{
		{"h1 within header", "# Early\n" + pad(10000), "Early", "", 0},
		{"h1 beyond header", pad(5000) + "# Late\n", "", "", 1},
		{"unclosed frontmatter falls back to canonical", "---\nname: ARCHITECTURE.md\n" + strings.Repeat("k: v\n", 3000) + "description: late\n---\n# Late\n", "", "CANONICAL", 1},
		{"partial last line is never interpreted", pad(4094) + "# Title text\n", "", "", 1},
		{"small file never counts as truncated", "no heading here\n", "", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			file := "NOTE.md"
			if tc.wantDesc == "CANONICAL" {
				file = "ARCHITECTURE.md"
				tc.wantDesc, _ = projectdocdomain.DocMetaDescription(file)
			}
			writeProjectDoc(t, repo, file, tc.content)
			catalog, omissions, _ := DiscoverProjectDocsReport(repo)
			if len(catalog) != 1 {
				t.Fatalf("catalog = %+v", catalog)
			}
			if catalog[0].Title != tc.wantTitle || catalog[0].Description != tc.wantDesc || omissions.HeaderTruncated != tc.wantTrunc {
				t.Fatalf("title=%q desc=%q header_truncated=%d, want %q %q %d", catalog[0].Title, catalog[0].Description, omissions.HeaderTruncated, tc.wantTitle, tc.wantDesc, tc.wantTrunc)
			}
		})
	}
}

type failingDirReader struct {
	batches [][]os.DirEntry
	calls   int
}

func (r *failingDirReader) ReadDir(int) ([]os.DirEntry, error) {
	r.calls++
	if r.calls <= len(r.batches) {
		return r.batches[r.calls-1], nil
	}
	return nil, errors.New("directory read failed")
}

type fakeDirEntry struct{ name string }

func (e fakeDirEntry) Name() string               { return e.name }
func (e fakeDirEntry) IsDir() bool                { return false }
func (e fakeDirEntry) Type() os.FileMode          { return 0 }
func (e fakeDirEntry) Info() (os.FileInfo, error) { return nil, errors.New("unused") }

func TestScanProjectDocNamesMarksReadErrorAsTruncated(t *testing.T) {
	batch := make([]os.DirEntry, 0, projectDocCatalogDirBatch)
	for index := 0; index < projectDocCatalogDirBatch; index++ {
		batch = append(batch, fakeDirEntry{name: fmt.Sprintf("doc-%03d.md", index)})
	}
	selector, batches, truncated := scanProjectDocNames(&failingDirReader{batches: [][]os.DirEntry{batch}})
	if !truncated || batches != 1 {
		t.Fatalf("truncated=%v batches=%d, want true and 1", truncated, batches)
	}
	if got := len(selector.names()); got != projectDocCatalogMaxEntries || selector.overCap != projectDocCatalogDirBatch-projectDocCatalogMaxEntries {
		t.Fatalf("partial result must be kept: names=%d over_cap=%d", got, selector.overCap)
	}
}

func TestFormatProjectDocCatalogUsesDescription(t *testing.T) {
	catalog := []projectdocdomain.ProjectDocCatalogEntry{
		{RelPath: ".issueops/ADR.md", Title: "구현 계획", Description: "Structural decisions, rationale, and rejected alternatives."},
		{RelPath: ".issueops/X.md", Title: "엑스", Description: ""},
	}
	got := FormatProjectDocCatalog(catalog)
	if !strings.HasPrefix(got, "project docs (read what's relevant): ") {
		t.Fatalf("unexpected catalog prefix: %q", got)
	}
	if !strings.Contains(got, "ADR.md=Structural decisions, rationale, and rejected alternatives.") {
		t.Fatalf("catalog should use description: %s", got)
	}
	if !strings.Contains(got, "X.md=엑스") {
		t.Fatalf("catalog should fall back to title when no description: %s", got)
	}
}

func TestFormatProjectDocCatalogEmpty(t *testing.T) {
	if got := FormatProjectDocCatalog(nil); got != "" {
		t.Fatalf("expected empty string for empty catalog, got %q", got)
	}
}

func writeProjectDoc(t *testing.T, repo, name, content string) {
	t.Helper()
	dir := filepath.Join(repo, ".issueops")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
