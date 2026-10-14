package projectdoc

import (
	"reflect"
	"testing"
)

func TestMatchRecordsNeedsTwoSharedTermsAndRanksRarerFirst(t *testing.T) {
	records := []Record{
		{Rel: "cautions/2026-01-01-a.md", Title: "Docker build timeout"},
		{Rel: "cautions/2026-01-02-b.md", Title: "Docker image extraction filled the disk"},
		{Rel: "cautions/2026-01-03-c.md", Title: "Docker logs rotate"},
		{Rel: "cautions/2026-01-04-d.md", Title: "Docker images and builds share the disk"},
	}
	var got []string
	for _, doc := range MatchRecords("이미지 빌드가 디스크를 채웠다 docker", records, 5) {
		got = append(got, doc.Rel)
	}
	// d shares image, build, disk, docker; b and a share fewer; c only docker.
	want := []string{"cautions/2026-01-04-d.md", "cautions/2026-01-02-b.md", "cautions/2026-01-01-a.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if docs := MatchRecords("docker", records, 5); len(docs) != 0 {
		t.Fatalf("a single shared term must not match: %v", docs)
	}
	if docs := MatchRecords("이미지 빌드가 디스크를 채웠다 docker", records, 1); len(docs) != 1 || docs[0].Rel != want[0] {
		t.Fatalf("limit not applied: %v", docs)
	}
}

func TestMatchRecordsPrefersNewerRecordOnTie(t *testing.T) {
	records := []Record{
		{Rel: "adr/2026-01-01-old.md", Title: "Lease token rotation"},
		{Rel: "adr/2026-02-01-new.md", Title: "Lease token rotation"},
	}
	docs := MatchRecords("rotate the lease token", records, 5)
	if len(docs) != 2 || docs[0].Rel != "adr/2026-02-01-new.md" {
		t.Fatalf("docs=%v", docs)
	}
}

func TestParseRecordStripsDateAndFallsBackToDescription(t *testing.T) {
	got := ParseRecord("cautions/2026-07-02-x.md", "---\nname: x\ndescription: Dated lesson about locks.\n---\n\n# 2026-07-02 — Re-verify locks\n\n- Source: review\n")
	want := Record{Rel: "cautions/2026-07-02-x.md", Title: "Re-verify locks", Summary: "Dated lesson about locks.", Source: "review"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestIsRecordFile(t *testing.T) {
	for name, want := range map[string]bool{
		"2026-09-07-reserve-disk.md": true,
		"overview.md":                false,
		"roadmap.md":                 false,
		"2026-13-07-bad-month.md":    false,
		"2026-09-07-notes.txt":       false,
	} {
		if got := IsRecordFile(name); got != want {
			t.Errorf("IsRecordFile(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestMatchModulesReachesTheModuleThatOwnsTheRuleByHeading(t *testing.T) {
	modules := []Record{
		ParseModule("testing/unit.md", "# Unit tests\n\n## Go race detector\n\n## Golden contract regeneration\n"),
		ParseModule("testing/hosts.md", "---\ndescription: Host parity for hooks and MCP registration.\n---\n# Hosts\n"),
		ParseModule("testing/api.md", "# API documentation checks\n\n## OpenAPI prompt\n"),
	}
	docs := MatchModules("golden contract drift after a race fix", modules, 3)
	if len(docs) != 1 || docs[0].Rel != "testing/unit.md" {
		t.Fatalf("headings must route to the owning module: %v", docs)
	}
	if docs := MatchModules("hook mcp registration", modules, 3); len(docs) != 1 || docs[0].Rel != "testing/hosts.md" {
		t.Fatalf("frontmatter description must route to the owning module: %v", docs)
	}
}

func TestMatchModulesIgnoresTermsTooManyModulesCarry(t *testing.T) {
	// "issueops" is in every module, so it cannot be the second shared term.
	modules := []Record{
		{Rel: "cautions/a.md", Title: "issueops worktree cleanup"},
		{Rel: "cautions/b.md", Title: "issueops lease fence"},
		{Rel: "cautions/c.md", Title: "issueops orchestration mailbox"},
	}
	if docs := MatchModules("issueops worktree", modules, 3); len(docs) != 0 {
		t.Fatalf("a common term must not count toward the two: %v", docs)
	}
	if docs := MatchRecords("issueops worktree", modules, 3); len(docs) != 1 {
		t.Fatalf("records keep counting every shared term: %v", docs)
	}
}
