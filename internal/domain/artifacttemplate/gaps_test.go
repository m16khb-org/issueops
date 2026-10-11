package artifacttemplate

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

func TestOptionalSectionTitlesFollowContractOrderPerTemplate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kind     IssueOpsArtifactKind
		template IssueOpsTemplateKind
		want     []string
	}{
		{"bug", IssueOpsArtifactIssue, IssueOpsTemplateBug, []string{"원인", "범위", "환경과 로그"}},
		{"proposal", IssueOpsArtifactIssue, IssueOpsTemplateProposal, []string{"완료 기준", "위험", "열린 질문"}},
		{"child defaults from kind", IssueOpsArtifactChild, "", []string{"검증"}},
		{"pr", IssueOpsArtifactPR, IssueOpsTemplatePullRequest, []string{"대안과 선택 이유", "위험과 되돌리기", "호환성과 마이그레이션", "남은 일"}},
		{"blank kind is an implementation issue", "", "", []string{"접근 방법과 대안", "위험", "열린 질문", "하위 Task"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := OptionalSectionTitles(tc.kind, tc.template); !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseFieldAssignmentsTrimsAndRejectsMalformed(t *testing.T) {
	got, err := ParseFieldAssignments([]string{" summary = 요약 = 본문 ", "empty="})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"summary": "요약 = 본문", "empty": ""}
	if !maps.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, bad := range []string{"no-equals", "=value", "  =value"} {
		if _, err := ParseFieldAssignments([]string{"ok=1", bad}); err == nil || !strings.Contains(err.Error(), "field must be key=value: "+bad) {
			t.Fatalf("ParseFieldAssignments(%q) error = %v", bad, err)
		}
	}
}

func TestRenderNormalizesKindTemplateProviderAndDefaults(t *testing.T) {
	fields := map[string]string{"summary": "요약", "changes": "변경", "verified": "확인", "reviewer_focus": "봐 주세요"}
	pr := Render(IssueOpsTemplateInput{Kind: " PR ", Provider: " GitHub ", Title: " 제목 ", Fields: fields})
	if !pr.OK || pr.Kind != IssueOpsArtifactPR || pr.Template != IssueOpsTemplatePullRequest || pr.Provider != "github" || pr.Title != "제목" {
		t.Fatalf("pr defaults = %+v", pr)
	}
	child := Render(IssueOpsTemplateInput{Kind: IssueOpsArtifactChild, Provider: "gitlab", Title: "c", Fields: map[string]string{
		"summary": "요약", "acceptance": "기준", "scope": "범위", "merge_condition": "선행 없음 아님",
	}})
	if !child.OK || child.Template != IssueOpsTemplateChildTask {
		t.Fatalf("child defaults = %+v", child)
	}
	issue := Render(IssueOpsTemplateInput{Provider: "github", Title: "i"})
	if issue.Kind != IssueOpsArtifactIssue || issue.Template != IssueOpsTemplateImplementationTask || issue.OK {
		t.Fatalf("issue defaults = %+v", issue)
	}
	if want := []string{"acceptance", "background", "scope", "summary", "verification"}; !slices.Equal(issue.MissingRequiredFields, want) {
		t.Fatalf("missing = %v, want %v", issue.MissingRequiredFields, want)
	}
	if !slices.Contains(issue.Validation.Critical, "missing_required_fields") {
		t.Fatalf("empty rendered body must be critical: %v", issue.Validation.Critical)
	}
}

func TestBugRenderFoldsFieldKeysAndMergesNormalizedDuplicates(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind: IssueOpsArtifactIssue, Template: IssueOpsTemplateBug, Provider: "github", Title: "버그",
		Fields: map[string]string{
			"summary":             "요약",
			"Reproduction Steps":  "1단계",
			"reproduction-steps":  "2단계",
			"expected_behavior":   "정상 응답",
			"Actual-Behavior":     "오류 응답",
			"acceptance":          "기준",
			"verification":        "검증 방법",
			"environment":         "macOS",
			"logs":                "panic 로그",
			"ignored_blank_field": "   ",
		},
	})
	if !result.OK || len(result.Warnings) != 0 {
		t.Fatalf("result = %+v", result)
	}
	for _, want := range []string{
		"## 재현 절차\n\n1단계\n\n2단계",
		"## 기대 동작과 실제 동작\n\n기대: 정상 응답\n실제: 오류 응답",
		"## 환경과 로그\n\n환경: macOS\n로그: panic 로그",
	} {
		if !strings.Contains(result.Body, want) {
			t.Fatalf("body missing %q:\n%s", want, result.Body)
		}
	}
	if strings.Contains(result.Body, "## 원인") {
		t.Fatalf("empty optional section must not render:\n%s", result.Body)
	}
}

func TestBugMissingExpectedActualNamesBothUnderlyingFields(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind: IssueOpsArtifactIssue, Template: IssueOpsTemplateBug, Provider: "github", Title: "버그",
		Fields: map[string]string{"summary": "요약", "reproduction_steps": "절차", "acceptance": "기준", "verification": "검증"},
	})
	if result.OK || !slices.Equal(result.MissingRequiredFields, []string{"actual_behavior", "expected_behavior"}) {
		t.Fatalf("missing = %v ok=%v", result.MissingRequiredFields, result.OK)
	}
}

func TestValidateAcceptsRequiredFieldSuppliedOnlyThroughBodySection(t *testing.T) {
	body := "## 요약\n\n요약\n\n## 재현 절차\n\n절차\n\n## 기대 동작과 실제 동작\n\n기대는 A, 실제는 B\n\n## 완료 기준\n\n기준\n\n## 검증\n\n검증"
	got := Validate(IssueOpsTemplateInput{Kind: IssueOpsArtifactIssue, Template: IssueOpsTemplateBug, Provider: "github", Title: "t", Body: body})
	if !got.OK || len(got.MissingRequiredFields) != 0 {
		t.Fatalf("body-supplied sections must satisfy required fields: %+v", got)
	}
	// A body lacking one of those sections does not satisfy its field.
	partial := strings.Replace(body, "## 검증\n\n검증", "", 1)
	got = Validate(IssueOpsTemplateInput{Kind: IssueOpsArtifactIssue, Template: IssueOpsTemplateBug, Provider: "github", Title: "t", Body: partial})
	if got.OK || !slices.Equal(got.MissingRequiredFields, []string{"verification"}) || !slices.Equal(got.MissingRequiredSections, []string{"검증"}) {
		t.Fatalf("partial body = %+v", got)
	}
}

func TestValidateRequiresKoreanBodyAndTitle(t *testing.T) {
	got := Validate(IssueOpsTemplateInput{
		Kind: IssueOpsArtifactPR, Template: IssueOpsTemplatePullRequest, Provider: "github", Title: "  ",
		Body: "## Summary\n\nEnglish only body",
	})
	for _, want := range []string{"korean_body_required", "missing_required_fields"} {
		if !slices.Contains(got.Critical, want) {
			t.Fatalf("critical %v missing %q", got.Critical, want)
		}
	}
	if !slices.Contains(got.MissingRequiredFields, "title") {
		t.Fatalf("blank title must be a missing field: %v", got.MissingRequiredFields)
	}
	if got.OK {
		t.Fatal("must not be OK")
	}
}

func TestValidateWarnsWhenProviderNotSet(t *testing.T) {
	got := Validate(IssueOpsTemplateInput{
		Kind: IssueOpsArtifactPR, Title: "t",
		Fields: map[string]string{"summary": "요약", "changes": "변경", "verified": "확인", "reviewer_focus": "봐 주세요"},
	})
	if !got.OK || !slices.Equal(got.Warnings, []string{"provider_not_set"}) {
		t.Fatalf("validation = %+v", got)
	}
}
