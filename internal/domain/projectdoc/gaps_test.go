package projectdoc

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestCatalogOmissionsAnyAndSummary(t *testing.T) {
	if (CatalogOmissions{}).Any() || (CatalogOmissions{}).Summary() != "" {
		t.Fatal("zero omissions must report nothing")
	}
	got := CatalogOmissions{Oversize: 2, OverCap: 1, HeaderTruncated: 3, ScanTruncated: true}
	if !got.Any() {
		t.Fatal("non-zero omissions must be Any")
	}
	if want := "oversize=2 over_cap=1 header_truncated=3 scan_truncated"; got.Summary() != want {
		t.Fatalf("Summary = %q, want %q", got.Summary(), want)
	}
	if only := (CatalogOmissions{Unreadable: 4}); !only.Any() || only.Summary() != "unreadable=4" {
		t.Fatalf("unreadable-only summary = %q", only.Summary())
	}
	if scan := (CatalogOmissions{ScanTruncated: true}); !scan.Any() || scan.Summary() != "scan_truncated" {
		t.Fatalf("scan-only summary = %q", scan.Summary())
	}
}

func TestPrefixedKnownProjectDocNamesIncludesRequiredAndOptionalDocs(t *testing.T) {
	names := PrefixedKnownProjectDocNames()
	for _, want := range []string{".issueops/ADR.md", ".issueops/TESTING.md", ".issueops/VCS.md", ".issueops/DESIGN.md"} {
		if !slices.Contains(names, want) {
			t.Fatalf("missing %s in %v", want, names)
		}
	}
	for _, name := range names {
		if !strings.HasPrefix(name, ".issueops/") {
			t.Fatalf("unprefixed name %q", name)
		}
	}
}

func TestNormalizeRouteTask(t *testing.T) {
	if got := NormalizeRouteTask("  \t\n"); got != "general" {
		t.Fatalf("blank task = %q, want general", got)
	}
	if got := NormalizeRouteTask("  Fix The BUG "); got != "fix the bug" {
		t.Fatalf("task = %q", got)
	}
}

func TestRecordModuleDirsNamesAppendOnlyFamilies(t *testing.T) {
	dirs := RecordModuleDirs()
	for _, dir := range dirs {
		if _, ok := FamilyByModuleDir(dir); !ok {
			t.Fatalf("record dir %q is not a doc family module", dir)
		}
	}
	if !slices.Contains(dirs, "adr") || !slices.Contains(dirs, "cautions") {
		t.Fatalf("dirs = %v", dirs)
	}
}

func TestRecordIndexLineEscapesBracketsInTitle(t *testing.T) {
	got := RecordIndexLine("  Fix [urgent] bug ", "2026-01-01-fix.md")
	if want := `- [Fix \[urgent\] bug](2026-01-01-fix.md)`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestParseRecordTitleDateStrippingAndSummaryFallback(t *testing.T) {
	explicit := ParseRecord("a.md", "description: fm\n# Short\n- Summary: explicit\n")
	if explicit.Summary != "explicit" {
		t.Fatalf("an explicit Summary line must beat frontmatter: %+v", explicit)
	}
	// A title shorter than a date or with a non-date prefix is kept whole.
	for title, want := range map[string]string{"Tiny": "Tiny", "2026-99-99 not a date": "2026-99-99 not a date"} {
		if got := ParseRecord("a.md", "# "+title+"\n").Title; got != want {
			t.Fatalf("title %q parsed as %q, want %q", title, got, want)
		}
	}
}

func TestParseRecordWithoutSummaryFallsBackToTheDecision(t *testing.T) {
	old := ParseRecord("adr/a.md", "# Task gate ledger\n\n## Context\nwhy\n\n## Decision\n\n1. CLI와 MCP가 같은 gates contract DTO를 공유한다.\n2. exit code 0/1/2.\n\n## Consequences\nlater\n")
	if old.Summary != "1. CLI와 MCP가 같은 gates contract DTO를 공유한다. 2. exit code 0/1/2." {
		t.Fatalf("summary = %q, want the Decision section text only", old.Summary)
	}
	inline := ParseRecord("adr/b.md", "# Title\n\n**결정:** hook은 정적 catalog만 렌더한다.\n\n## 결정 근거\nnot this\n")
	if inline.Summary != "hook은 정적 catalog만 렌더한다." {
		t.Fatalf("summary = %q, want the inline 결정 line", inline.Summary)
	}
	long := ParseRecord("adr/c.md", "# T\n## Decision\n"+strings.Repeat("가", 400)+"\n")
	if got := utf8.RuneCountInString(long.Summary); got != decisionFallbackRunes {
		t.Fatalf("summary runes = %d, want the %d-rune cap so long decisions do not match every task", got, decisionFallbackRunes)
	}
	for content, want := range map[string]string{
		"# T\n\n**결정: 외부 코어와 얇은 adapter를 쓴다.**\n\n- 근거\n":                                              "외부 코어와 얇은 adapter를 쓴다.",
		"# T\n\nDecision: prefer a tarball first.\nDefer Homebrew.\n\nlater text\n":                    "prefer a tarball first. Defer Homebrew.",
		"# T\n\n- Kind: `adr`\n- Decision: port may import\n  contract DTOs.\n- Rationale: not this\n": "port may import contract DTOs.",
		"# T\n\n## Summary\n\nHTTP 설치 검증이 stdio만 본다.\n\n## Fix\nnot this\n":                            "HTTP 설치 검증이 stdio만 본다.",
	} {
		if got := ParseRecord("adr/e.md", content).Summary; got != want {
			t.Fatalf("summary = %q, want %q for %q", got, want, content)
		}
	}
	if described := ParseRecord("adr/d.md", "description: fm\n# T\n## Decision\nbody\n"); described.Summary != "fm" {
		t.Fatalf("summary = %q, want frontmatter description to win over the decision fallback", described.Summary)
	}
}

func TestMatchRecordsIgnoresPlanAndProblemFiller(t *testing.T) {
	records := []Record{
		{Rel: "adr/lint.md", Title: "gates lint", Summary: "계획 파일의 게이트를 검사해 리뷰 문제를 줄인다. MCP tool."},
		{Rel: "adr/mcp.md", Title: "shared MCP service", Summary: "update restarts the MCP service build."},
	}
	got := MatchRecords("update 뒤 MCP 서비스가 이전 build로 남는 문제 수정 계획", records, 5)
	if len(got) != 1 || got[0].Rel != "adr/mcp.md" {
		t.Fatalf("routes = %+v, want only the MCP service record; sharing mcp plus 계획/문제 filler is not a match", got)
	}
}

func TestValidateRevisionInputRejectsBlankContentAndSummary(t *testing.T) {
	if err := ValidateRevisionInput(RevisionInput{Content: " \n\n", Summary: "s"}); err == nil || !strings.Contains(err.Error(), "content is required") {
		t.Fatalf("blank content error = %v", err)
	}
	if err := ValidateRevisionInput(RevisionInput{Content: "# x", Summary: "  "}); err == nil || !strings.Contains(err.Error(), "summary is required") {
		t.Fatalf("blank summary error = %v", err)
	}
	if err := ValidateRevisionInput(RevisionInput{Content: "# x", Summary: "s"}); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
}

func TestClassifyVCSProviderFromHostSubstring(t *testing.T) {
	for _, tc := range []struct {
		origin, provider, hosting, host string
	}{
		{"https://gitlab.com/o/r", "gitlab", "managed", "gitlab.com"},
		{"git@bitbucket.org:o/r.git", "bitbucket", "managed", "bitbucket.org"},
		{"https://Code.GitHub.Corp/o/r", "github", "self-hosted", "code.github.corp"},
		{"https://bitbucket.corp.io/o/r", "bitbucket", "self-hosted", "bitbucket.corp.io"},
		{"https://git.example.org/o/r", "git", "self-hosted", "git.example.org"},
	} {
		got := ClassifyVCS(tc.origin, true)
		if got.Provider != tc.provider || got.Hosting != tc.hosting || got.RemoteHost != tc.host || got.RemoteName != "origin" {
			t.Errorf("ClassifyVCS(%q) = %+v", tc.origin, got)
		}
	}
}

func TestRemoteHostForms(t *testing.T) {
	for remote, want := range map[string]string{
		"":                                  "",
		"   ":                               "",
		"ssh://git@Host.Example.com:2222/x": "host.example.com",
		"git@GitLab.Example.com:grp/repo":   "gitlab.example.com",
		"git@host.example.com/grp/repo":     "host.example.com",
		"git@host-only":                     "",
		"relative/path":                     "",
	} {
		if got := RemoteHost(remote); got != want {
			t.Errorf("RemoteHost(%q) = %q, want %q", remote, got, want)
		}
	}
}

func TestRecordSlugNormalizesAndFallsBack(t *testing.T) {
	for title, want := range map[string]string{
		"  Fix: the DISK!! ": "fix-the-disk",
		"한글만":                "record",
		"---":                "record",
		"a/b  c":             "a-b-c",
	} {
		if got := RecordSlug(title); got != want {
			t.Errorf("RecordSlug(%q) = %q, want %q", title, got, want)
		}
	}
	if got := RecordSlug(strings.Repeat("a", 100)); len(got) != 60 {
		t.Fatalf("slug length = %d, want capped at 60", len(got))
	}
}

func TestIsFamilyDocRel(t *testing.T) {
	for rel, want := range map[string]bool{
		".issueops/ADR.md":                 true,
		"TESTING.md":                       true,
		".issueops/adr/overview.md":        true,
		"operations/guides/deploy.md":      true,
		"AGENTS.md":                        false,
		".issueops/VCS.md":                 false,
		".issueops/adrenaline/overview.md": false,
	} {
		if got := IsFamilyDocRel(rel); got != want {
			t.Errorf("IsFamilyDocRel(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestNormalizeRelPathRejectsEmptyAndUnknownDocs(t *testing.T) {
	if _, err := NormalizeRelPath("  ./ "); err == nil || !strings.Contains(err.Error(), "rel_path is required") {
		t.Fatalf("empty error = %v", err)
	}
	if _, err := NormalizeRelPath("secrets/passwd.md"); err == nil || !strings.Contains(err.Error(), "unsupported project doc") {
		t.Fatalf("unknown doc error = %v", err)
	}
}

func TestEnsureMetaFrontmatterEdgeCases(t *testing.T) {
	empty := EnsureMetaFrontmatter("ADR.md", "  \n")
	if !strings.HasPrefix(empty, "---\nname: ADR.md\n") || !strings.HasSuffix(empty, "---\n") {
		t.Fatalf("empty body must yield only the block: %q", empty)
	}
	replaced := EnsureMetaFrontmatter("ADR.md", "---\nname: stale\ndescription: old\n---\n\n\n# Body\n")
	if strings.Contains(replaced, "stale") || !strings.HasSuffix(replaced, "---\n\n# Body\n") {
		t.Fatalf("stale frontmatter not replaced cleanly: %q", replaced)
	}
	// An unterminated block is not frontmatter, so it stays in the body.
	const unterminated = "---\nname: x\n# no close\n"
	if _, _, body, ok := ParseFrontmatter(unterminated); ok || body != unterminated {
		t.Fatalf("unterminated block ok=%v body=%q", ok, body)
	}
}
