package issuebody

import (
	"regexp"
	"strings"
	"testing"

	"issueops/internal/port"
)

var sha256Hex = regexp.MustCompile(`[0-9a-f]{64}`)

func TestMergeManagedSectionIdempotent(t *testing.T) {
	start, end, err := SectionMarkers(SectionDevilsAdvocate)
	if err != nil {
		t.Fatal(err)
	}
	sec := RenderDevilsAdvocateSection([]string{"gold-plating", "schedule optimism", "  "}, "2026-07-01T00:00:00Z")
	body := "original body\n"

	once := MergeManagedSection(body, sec, start, end)
	if !strings.Contains(once, "gold-plating") || !strings.HasPrefix(once, "original body") {
		t.Fatalf("append failed: %q", once)
	}
	if strings.Contains(once, "- \n") {
		t.Fatalf("blank finding should be dropped: %q", once)
	}

	sec2 := RenderDevilsAdvocateSection([]string{"new finding"}, "2026-07-02T00:00:00Z")
	twice := MergeManagedSection(once, sec2, start, end)
	if strings.Count(twice, start) != 1 || strings.Count(twice, end) != 1 {
		t.Fatalf("re-merge must not duplicate the block: %q", twice)
	}
	if strings.Contains(twice, "gold-plating") || !strings.Contains(twice, "new finding") {
		t.Fatalf("re-merge must replace the block content: %q", twice)
	}
	if !strings.HasPrefix(twice, "original body") {
		t.Fatalf("surrounding body must round-trip: %q", twice)
	}
}

func TestMergeManagedSectionEmptyBody(t *testing.T) {
	start, end, err := SectionMarkers(SectionDevilsAdvocate)
	if err != nil {
		t.Fatal(err)
	}
	sec := RenderDevilsAdvocateSection([]string{"x"}, "t")
	got := MergeManagedSection("", sec, start, end)
	if !strings.Contains(got, "x") || !strings.HasPrefix(got, start) {
		t.Fatalf("empty body should become just the section: %q", got)
	}
}

func TestSectionMarkersRejectsUnknownKind(t *testing.T) {
	if _, _, err := SectionMarkers("release-notes"); err == nil {
		t.Fatal("unknown section kind must be rejected")
	}
}

func completionFixture() port.IssueProviderCompletionSection {
	return port.IssueProviderCompletionSection{
		RemoteArtifactURL: "https://github.com/acme/repo/pull/9",
		ResultBody: "두 이슈를 서로 다른 세션에서 동시에 진행해도 간섭하지 않음을 확인했습니다.\n\n" +
			"- 계획: 한 사이클을 끝까지 실행한다.\n- 구현: 보고서를 작성했다(PR #9).",
	}
}

// 진행 결과 구간은 사람이 쓴 원고만 담는다. 하네스 값(해시, 최종 head, manifest,
// plan·spec 전문, 빈 소제목)은 렌더하지 않는다(#513).
func TestCompletionSectionIsHumanReadable(t *testing.T) {
	got := RenderCompletionSection(completionFixture())
	if !strings.HasPrefix(got, CompletionStartMarker+"\n") || !strings.HasSuffix(got, completionEndMarker) {
		t.Fatalf("the managed region keeps both markers: %q", got)
	}
	if !strings.Contains(got, "## 진행 결과\n") || !strings.Contains(got, completionFixture().ResultBody) {
		t.Fatalf("the region renders the heading and the written result: %q", got)
	}
	for _, leaked := range []string{"###", "(없음)", "<details>", "plan 전문", "/Users/", "완료 기록"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("the region must not render %q: %q", leaked, got)
		}
	}
	if sha256Hex.MatchString(got) {
		t.Fatalf("the region must not render a 64-digit hex: %q", got)
	}
}

func TestRenderSectionRefusesCompletionOverBudget(t *testing.T) {
	c := completionFixture()
	c.ResultBody = strings.Repeat("긴 원고입니다. ", 200)
	if _, _, _, err := RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{Section: SectionCompletion, Completion: &c}, "t", 100); err == nil {
		t.Fatal("a result over the body budget must fail instead of being cut")
	}
}

func TestRenderSectionRoutesByKind(t *testing.T) {
	if _, _, _, err := RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{Section: SectionCompletion}, "t", 0); err == nil {
		t.Fatal("completion section without payload must be rejected")
	}
	section, start, _, err := RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{
		Section: SectionCompletion, Completion: &port.IssueProviderCompletionSection{},
	}, "t", 0)
	if err != nil || !strings.HasPrefix(section, start) || start != CompletionStartMarker {
		t.Fatalf("completion render failed: %v %q", err, section)
	}
	section, start, _, err = RenderSection(port.IssueProviderUpdateIssueBodySectionRequest{
		Section: SectionDevilsAdvocate, Findings: []string{"f"},
	}, "t", 0)
	if err != nil || !strings.HasPrefix(section, start) {
		t.Fatalf("devils-advocate render failed: %v %q", err, section)
	}
}

// 두 섹션은 서로의 블록을 건드리지 않아야 한다.
func TestCompletionAndDevilsAdvocateSectionsCoexist(t *testing.T) {
	daStart, daEnd, _ := SectionMarkers(SectionDevilsAdvocate)
	coStart, coEnd, _ := SectionMarkers(SectionCompletion)
	body := MergeManagedSection("base\n", RenderDevilsAdvocateSection([]string{"finding"}, "t"), daStart, daEnd)
	body = MergeManagedSection(body, RenderCompletionSection(completionFixture()), coStart, coEnd)
	if strings.Count(body, daStart) != 1 || strings.Count(body, coStart) != 1 {
		t.Fatalf("both sections must coexist exactly once: %q", body)
	}
	body2 := MergeManagedSection(body, RenderCompletionSection(completionFixture()), coStart, coEnd)
	if !strings.Contains(body2, "finding") || strings.Count(body2, coStart) != 1 {
		t.Fatalf("re-merging completion must preserve devils-advocate block: %q", body2)
	}
}
