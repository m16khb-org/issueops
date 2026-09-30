package issueops

import (
	"regexp"
	"strings"
	"testing"

	completionmodel "issueops/internal/contract/issueops"
)

var sha256Hex = regexp.MustCompile(`[0-9a-f]{64}`)

func TestMergeManagedSectionIdempotent(t *testing.T) {
	start, end, err := SectionMarkers(completionmodel.IssueBodySectionDevilsAdvocate)
	if err != nil {
		t.Fatal(err)
	}
	sec := RenderDevilsAdvocateSection(stopReview("gold-plating", "schedule optimism", "  "))
	body := "original body\n"

	once := MergeManagedSection(body, sec, start, end)
	if !strings.Contains(once, "gold-plating") || !strings.HasPrefix(once, "original body") {
		t.Fatalf("append failed: %q", once)
	}
	if strings.Contains(once, "- \n") {
		t.Fatalf("blank finding should be dropped: %q", once)
	}

	sec2 := RenderDevilsAdvocateSection(stopReview("new finding"))
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
	start, end, err := SectionMarkers(completionmodel.IssueBodySectionDevilsAdvocate)
	if err != nil {
		t.Fatal(err)
	}
	sec := RenderDevilsAdvocateSection(stopReview("x"))
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

func completionFixture() completionmodel.RemoteCompletionSection {
	return completionmodel.RemoteCompletionSection{
		RemoteArtifactURL: "https://github.com/acme/repo/pull/9",
		ResultBody: "두 이슈를 서로 다른 세션에서 동시에 진행해도 간섭하지 않음을 확인했습니다.\n\n" +
			"- 계획: 한 사이클을 끝까지 실행한다.\n- 구현: 보고서를 작성했다(PR #9).",
	}
}

// 진행 결과 구간은 사람이 쓴 원고만 담는다. 하네스 값(해시, 최종 head, manifest,
// plan·spec 전문, 빈 소제목)은 렌더하지 않는다(#513).
func TestCompletionSectionIsHumanReadable(t *testing.T) {
	got := RenderCompletionSection(completionFixture())
	if !strings.HasPrefix(got, completionmodel.IssueBodyCompletionStartMarker+"\n") || !strings.HasSuffix(got, completionEndMarker) {
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
	if _, _, _, err := RenderSection(SectionInput{Section: completionmodel.IssueBodySectionCompletion, Completion: &c}, 100); err == nil {
		t.Fatal("a result over the body budget must fail instead of being cut")
	}
}

func TestRenderSectionRoutesByKind(t *testing.T) {
	if _, _, _, err := RenderSection(SectionInput{Section: completionmodel.IssueBodySectionCompletion}, 0); err == nil {
		t.Fatal("completion section without payload must be rejected")
	}
	section, start, _, err := RenderSection(SectionInput{
		Section: completionmodel.IssueBodySectionCompletion, Completion: &completionmodel.RemoteCompletionSection{},
	}, 0)
	if err != nil || !strings.HasPrefix(section, start) || start != completionmodel.IssueBodyCompletionStartMarker {
		t.Fatalf("completion render failed: %v %q", err, section)
	}
	section, start, _, err = RenderSection(SectionInput{
		Section: completionmodel.IssueBodySectionDevilsAdvocate, Verdict: "stop", Findings: []string{"f"},
		Rounds: []completionmodel.PlanReviewRound{{Verdict: "stop", Findings: 1}},
	}, 0)
	if err != nil || !strings.HasPrefix(section, start) {
		t.Fatalf("devils-advocate render failed: %v %q", err, section)
	}
}

// 두 섹션은 서로의 블록을 건드리지 않아야 한다.
func TestCompletionAndDevilsAdvocateSectionsCoexist(t *testing.T) {
	daStart, daEnd, _ := SectionMarkers(completionmodel.IssueBodySectionDevilsAdvocate)
	coStart, coEnd, _ := SectionMarkers(completionmodel.IssueBodySectionCompletion)
	body := MergeManagedSection("base\n", RenderDevilsAdvocateSection(stopReview("finding")), daStart, daEnd)
	body = MergeManagedSection(body, RenderCompletionSection(completionFixture()), coStart, coEnd)
	if strings.Count(body, daStart) != 1 || strings.Count(body, coStart) != 1 {
		t.Fatalf("both sections must coexist exactly once: %q", body)
	}
	body2 := MergeManagedSection(body, RenderCompletionSection(completionFixture()), coStart, coEnd)
	if !strings.Contains(body2, "finding") || strings.Count(body2, coStart) != 1 {
		t.Fatalf("re-merging completion must preserve devils-advocate block: %q", body2)
	}
}

func stopReview(findings ...string) SectionInput {
	return SectionInput{
		Section: completionmodel.IssueBodySectionDevilsAdvocate, Verdict: "stop", Findings: findings,
		Rounds: []completionmodel.PlanReviewRound{{Verdict: "stop", Findings: len(findings)}},
	}
}

// 계획 검토 구간은 라운드를 흐름 한 줄로 요약하고, 통과했으면 지적 원문을
// 싣지 않는다. 지적 원문은 record와 plan-review.md에 있다(#513).
func TestPlanReviewSectionSummarizesRounds(t *testing.T) {
	got := RenderDevilsAdvocateSection(SectionInput{
		Section: completionmodel.IssueBodySectionDevilsAdvocate, Verdict: "pass", Findings: []string{"구현 메모"},
		Rounds: []completionmodel.PlanReviewRound{{Verdict: "revise", Findings: 3}, {Verdict: "pass", Findings: 1}},
	})
	if !strings.Contains(got, "## 계획 검토\n") {
		t.Fatalf("the region is headed 계획 검토: %q", got)
	}
	if !strings.Contains(got, "계획 검토: 1차 수정 요청(지적 3건) → 계획 수정 → 2차 통과(지적 1건)") {
		t.Fatalf("the rounds read as one flow line: %q", got)
	}
	if strings.Contains(got, "구현 메모") || strings.Contains(got, "Devil") {
		t.Fatalf("a passed review carries no finding text or English verdicts: %q", got)
	}
	stop := RenderDevilsAdvocateSection(stopReview("범위가 이슈와 다르다"))
	if !strings.Contains(stop, "1차 중단(지적 1건)") || !strings.Contains(stop, "- 범위가 이슈와 다르다") {
		t.Fatalf("a stop lists its reasons: %q", stop)
	}
}

func TestPlanReviewMasksHashesAndPaths(t *testing.T) {
	got := RenderDevilsAdvocateSection(stopReview("plan digest " + strings.Repeat("ab", 32) + "와 커밋 " + strings.Repeat("cd", 20) + ", /Users/x/plan.md를 확인"))
	if sha256Hex.MatchString(got) || strings.Contains(got, strings.Repeat("cd", 20)) || strings.Contains(got, "/Users/") {
		t.Fatalf("hashes and local paths must be masked: %q", got)
	}
	if !strings.Contains(got, "[해시 생략]") || !strings.Contains(got, "[로컬 경로 생략]") {
		t.Fatalf("masks must say what was left out: %q", got)
	}
}

// Replacing a managed block must not normalize authored bytes on either side.
func TestMergeManagedSectionPreservesSurroundingBytes(t *testing.T) {
	const body = "  사용자 본문\r\n\t<start>old\nvalue<end>\r\n  tail \n\n"
	const want = "  사용자 본문\r\n\t<start>new<end>\r\n  tail \n\n"
	if got := MergeManagedSection(body, "<start>new<end>", "<start>", "<end>"); got != want {
		t.Fatalf("merged = %q, want %q", got, want)
	}
}

func TestSectionBudgetUsesUnmanagedBytes(t *testing.T) {
	for _, tc := range []struct {
		name, body  string
		limit, want int
	}{
		{"unlimited", "prefix", 0, 0},
		{"replacement", "abc<s>old</s>xy", 20, 15},
		{"absent", "abc", 20, 17},
		{"unterminated", "abc<s>old", 20, 11},
		{"reversed", "</s>abc<s>", 20, 10},
		{"exhausted", "abc", 3, 1},
		{"overflow", "abcd", 3, 1},
		{"multibyte", "한", 10, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := SectionBudget(tc.body, tc.limit, "<s>", "</s>"); got != tc.want {
				t.Fatalf("budget = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestRenderSectionRejectsMandatoryContentBeyondBudget(t *testing.T) {
	_, _, _, err := RenderSection(SectionInput{Section: "completion", Completion: &completionmodel.RemoteCompletionSection{ResultBody: strings.Repeat("x", 2000)}}, 1000)
	if err == nil || !strings.Contains(err.Error(), "exceeds the body budget") {
		t.Fatalf("oversized mandatory content: %v", err)
	}
}
