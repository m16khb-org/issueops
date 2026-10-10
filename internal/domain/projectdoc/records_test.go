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
