package projectdoc

import (
	"maps"
	"slices"
	"testing"

	projectdoccontract "issueops/internal/contract/projectdoc"
)

func TestBuildProfileGitRemoteEvidenceByProvider(t *testing.T) {
	for _, tc := range []struct {
		provider string
		want     bool
	}{{"git", true}, {"local", true}, {"other", false}} {
		profile := BuildProfile(ProjectSignals{}, projectdoccontract.ProjectVCSProfile{Provider: tc.provider}, nil, false, nil, nil)
		if got := slices.Contains(profile.Evidence, "git remote/config"); got != tc.want {
			t.Errorf("provider %q: git evidence=%v want %v", tc.provider, got, tc.want)
		}
	}
}

func TestIsRecordFileMinimumLength(t *testing.T) {
	if !IsRecordFile("2026-09-07-x.md") {
		t.Error("shortest dated record name must be accepted")
	}
	if IsRecordFile("2026-09-07-.md") {
		t.Error("record name without slug must be rejected")
	}
}

func TestParseRecordTitleThatIsOnlyADate(t *testing.T) {
	if got := ParseRecord("cautions/x.md", "# 2026-09-07\n").Title; got != "" {
		t.Fatalf("title=%q, want empty", got)
	}
}

func TestMatchRecordsLimitAndTermBounds(t *testing.T) {
	records := []Record{{Rel: "adr/a.md", Title: "alpha bravo"}}
	if got := MatchRecords("alpha bravo", records, 0); got != nil {
		t.Errorf("limit 0 must return nil, got %#v", got)
	}
	if got := MatchRecords("alpha bravo", records, 1); len(got) != 1 {
		t.Errorf("exactly two task terms must match, got %#v", got)
	}
	if got := MatchRecords("alpha", records, 1); got != nil {
		t.Errorf("one task term must not match, got %#v", got)
	}
}

func TestMatchRecordsWeighsRareTermsAboveCommonOnes(t *testing.T) {
	records := []Record{
		{Rel: "adr/a.md", Title: "gamma delta"},
		{Rel: "adr/b.md", Title: "alpha bravo"},
		{Rel: "adr/c.md", Title: "alpha bravo"},
	}
	got := MatchRecords("alpha bravo gamma delta", records, 3)
	if len(got) != 3 || got[0].Rel != "adr/a.md" {
		t.Fatalf("record sharing rarer terms must rank first, got %#v", got)
	}
}

func TestRouteTermsSplitsLatinAndHangulRuns(t *testing.T) {
	got := routeTerms("deploy서버")
	want := map[string]bool{"deploy": true, "server": true}
	if !maps.Equal(got, want) {
		t.Fatalf("terms=%v want %v", got, want)
	}
}

func TestRouteTermsHangulStemLength(t *testing.T) {
	if got := routeTerms("고양"); !maps.Equal(got, map[string]bool{"고양": true}) {
		t.Errorf("two-syllable word must keep itself as the stem, got %v", got)
	}
	if got := routeTerms("가"); len(got) != 0 {
		t.Errorf("one-syllable word must yield no term, got %v", got)
	}
}

func TestRouteTermsDropsShortAndStopWords(t *testing.T) {
	got := routeTerms("ab the and deploy")
	if !maps.Equal(got, map[string]bool{"deploy": true}) {
		t.Fatalf("terms=%v, want only deploy", got)
	}
}

func TestMatchRecordsStemsInflectedLatinWords(t *testing.T) {
	records := []Record{{Rel: "adr/a.md", Title: "test flaky"}}
	if got := MatchRecords("testing tests flaky", records, 1); len(got) != 1 {
		t.Fatalf("inflected task words must reach the base-form record, got %#v", got)
	}
}

func TestMatchRecordsKeepsThreeLetterWords(t *testing.T) {
	records := []Record{{Rel: "adr/a.md", Title: "git dns"}}
	if got := MatchRecords("git dns", records, 1); len(got) != 1 {
		t.Fatalf("three-letter words must count as terms, got %#v", got)
	}
}
