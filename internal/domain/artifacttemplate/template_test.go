package artifacttemplate

import (
	"slices"
	"strings"
	"testing"
)

func TestPullRequestContractRequiresFourSections(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "github",
		Title:    "PR",
		Fields: map[string]string{
			"summary":        "본문 계약을 요약이 먼저 오는 형태로 바꿨습니다.",
			"changes":        "artifacttemplate 패키지의 절 구성을 새 계약으로 바꿨습니다.",
			"verified":       "go test ./internal/domain/artifacttemplate로 확인했습니다.",
			"reviewer_focus": "필수 절 4개가 맞는지 봐 주세요.",
		},
	})
	if !result.OK {
		t.Fatalf("PR with all four required sections should pass: %+v", result)
	}
	for _, want := range []string{"## 요약", "## 변경 내용", "## 확인한 것", "## 리뷰 포인트"} {
		if !strings.Contains(result.Body, want) {
			t.Fatalf("PR body missing required section %q:\n%s", want, result.Body)
		}
	}

	missingReviewerFocus := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "github",
		Title:    "PR",
		Fields: map[string]string{
			"summary":  "요약",
			"changes":  "변경",
			"verified": "확인",
		},
	})
	if missingReviewerFocus.OK {
		t.Fatalf("PR missing reviewer_focus must not be OK: %+v", missingReviewerFocus)
	}
	if !contains(missingReviewerFocus.MissingRequiredFields, "reviewer_focus") {
		t.Fatalf("missing fields %v did not include reviewer_focus", missingReviewerFocus.MissingRequiredFields)
	}
}

func TestImplementationIssueContractRequiresFiveSections(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateImplementationTask,
		Provider: "github",
		Title:    "본문 계약 강화",
		Fields: map[string]string{
			"summary":      "이슈 본문을 요약이 먼저 오는 짧은 계약으로 바꿉니다.",
			"background":   "지금 본문은 13절 고정 형식이라 팀원이 읽기 어렵습니다.",
			"acceptance":   "새 계약으로 게시한 이슈의 첫 절이 요약입니다.",
			"scope":        "artifacttemplate 패키지만 바꿉니다.",
			"verification": "go test ./internal/domain/artifacttemplate",
		},
	})
	if !result.OK {
		t.Fatalf("implementation_task with all five required sections should pass: %+v", result)
	}
	for _, want := range []string{"## 요약", "## 배경", "## 완료 기준", "## 범위", "## 검증"} {
		if !strings.Contains(result.Body, want) {
			t.Fatalf("issue body missing required section %q:\n%s", want, result.Body)
		}
	}

	missingScope := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateImplementationTask,
		Provider: "github",
		Title:    "본문 계약 강화",
		Fields: map[string]string{
			"summary":      "요약",
			"background":   "배경",
			"acceptance":   "완료 기준",
			"verification": "검증",
		},
	})
	if missingScope.OK {
		t.Fatalf("implementation_task missing scope must not be OK: %+v", missingScope)
	}
	if !contains(missingScope.MissingRequiredFields, "scope") {
		t.Fatalf("missing fields %v did not include scope", missingScope.MissingRequiredFields)
	}
}

func TestBugContractRequiresReproductionAndExpectedActual(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateBug,
		Provider: "github",
		Title:    "로그인 실패",
		Fields: map[string]string{
			"summary":            "로그인 시도가 500 오류로 실패합니다.",
			"reproduction_steps": "로그인 버튼을 누릅니다.",
			"expected_behavior":  "토큰이 발급됩니다.",
			"actual_behavior":    "500이 반환됩니다.",
			"acceptance":         "정상 계정으로 로그인됩니다.",
			"verification":       "go test ./internal/auth/...",
		},
	})
	if !result.OK {
		t.Fatalf("bug with all required fields should pass: %+v", result)
	}
	if !strings.Contains(result.Body, "## 기대 동작과 실제 동작") {
		t.Fatalf("bug body missing combined expected/actual section:\n%s", result.Body)
	}

	missing := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateBug,
		Provider: "github",
		Title:    "로그인 실패",
		Fields: map[string]string{
			"summary": "로그인 시도가 500 오류로 실패합니다.",
		},
	})
	if missing.OK {
		t.Fatalf("bug missing required fields must not be OK: %+v", missing)
	}
	for _, want := range []string{"reproduction_steps", "expected_behavior", "actual_behavior", "acceptance", "verification"} {
		if !contains(missing.MissingRequiredFields, want) {
			t.Fatalf("missing fields %v did not include %q", missing.MissingRequiredFields, want)
		}
	}
}

func TestProposalContractRequiresAlternativesAndScope(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateProposal,
		Provider: "github",
		Title:    "본문 계약 재설계 제안",
		Fields: map[string]string{
			"summary":                "본문 계약을 요약 우선으로 바꾸자는 제안입니다.",
			"background":             "지금 본문은 세 독자를 동시에 상대합니다.",
			"proposal":               "본문 계약을 종류별 4~5개 필수 절로 재정의합니다.",
			"alternatives_rationale": "스킬 지침만 정비하는 안은 다시 건너뛸 수 있어 기각했습니다.",
			"scope":                  "artifacttemplate과 게시 명령만 바꿉니다.",
		},
	})
	if !result.OK {
		t.Fatalf("proposal with all required fields should pass: %+v", result)
	}
	for _, want := range []string{"## 제안", "## 대안과 선택 이유"} {
		if !strings.Contains(result.Body, want) {
			t.Fatalf("proposal body missing %q:\n%s", want, result.Body)
		}
	}
}

func TestChildTaskContractRequiresMergeCondition(t *testing.T) {
	result := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactChild,
		Template: IssueOpsTemplateChildTask,
		Provider: "github",
		Title:    "하위 작업",
		Fields: map[string]string{
			"summary":         "부모 이슈 #1의 템플릿 렌더러 구현을 맡습니다.",
			"acceptance":      "렌더러 테스트가 통과합니다.",
			"scope":           "artifacttemplate 패키지만 바꿉니다.",
			"merge_condition": "부모 브랜치에 병합된 뒤 close-children을 실행합니다.",
		},
	})
	if !result.OK {
		t.Fatalf("child_task with all required fields should pass: %+v", result)
	}
	if !strings.Contains(result.Body, "## 선행 조건과 병합 조건") {
		t.Fatalf("child body missing merge condition section:\n%s", result.Body)
	}
}

func TestRenderTemplateAcceptsLegacyFieldAliases(t *testing.T) {
	pr := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "gitlab",
		Title:    "MR",
		Fields: map[string]string{
			"intent":         "원격 템플릿 계약을 고정합니다.",
			"changes":        "core renderer와 CLI/MCP를 추가합니다.",
			"verified":       "go test ./...",
			"reviewer_focus": "template validation boundary",
			"risks":          "기능 flag 없이 dry-run 기본 유지",
			"breaking":       "- [x] 없음",
			"user_impact":    "원격 artifact 품질 일관성 개선",
			"documentation":  "IssueOps 문서 갱신",
			"scope":          "provider adapter는 thin 유지",
			"cleanup":        "cleanup status 확인",
			"automation":     "AI 생성 본문은 renderer 결과로 검증",
		},
	})
	if !pr.OK {
		t.Fatalf("pr template should accept legacy field aliases: %+v", pr)
	}
	if !strings.Contains(pr.Body, "## 위험과 되돌리기") {
		t.Fatalf("pr body missing risk_rollback alias target:\n%s", pr.Body)
	}
	if !strings.Contains(pr.Body, "## 호환성과 마이그레이션") {
		t.Fatalf("pr body missing compatibility_migration alias target:\n%s", pr.Body)
	}
	if strings.Contains(pr.Body, "## 범위") || strings.Contains(pr.Body, "## 워크트리 정리") || strings.Contains(pr.Body, "## 자동화") {
		t.Fatalf("pr body must not render unrendered-field sections:\n%s", pr.Body)
	}

	child := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactChild,
		Template: IssueOpsTemplateChildTask,
		Provider: "github",
		Title:    "하위 작업",
		Fields: map[string]string{
			"parent_issue":    "https://github.com/acme/repo/issues/1",
			"task_goal":       "템플릿 렌더러 구현",
			"acceptance":      "렌더러 테스트 통과",
			"scope":           "provider 정책 복제 제외",
			"merge_condition": "부모 브랜치에 병합된 뒤 close-children 실행",
		},
	})
	if !child.OK {
		t.Fatalf("child template should accept parent_issue/task_goal aliases folded into summary: %+v", child)
	}
	if !strings.Contains(child.Body, "https://github.com/acme/repo/issues/1") || !strings.Contains(child.Body, "템플릿 렌더러 구현") {
		t.Fatalf("child summary section missing folded parent_issue/task_goal content:\n%s", child.Body)
	}
}

func TestValidateWarnsOnUnrenderedFields(t *testing.T) {
	validation := Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "github",
		Title:    "PR",
		Body: strings.Join([]string{
			"## 요약\n\n본문 계약을 새로 정의했습니다.",
			"## 변경 내용\n\nartifacttemplate 패키지를 바꿨습니다.",
			"## 확인한 것\n\ngo test로 확인했습니다.",
			"## 리뷰 포인트\n\n필수 절이 맞는지 봐 주세요.",
		}, "\n\n"),
		Fields: map[string]string{
			"worktree_cleanup": "워크트리 상태 확인",
		},
	})
	if !validation.OK {
		t.Fatalf("unrendered field must warn, not fail: %+v", validation)
	}
	if !contains(validation.Warnings, "unrendered_field:worktree_cleanup") {
		t.Fatalf("warnings %v missing unrendered_field:worktree_cleanup", validation.Warnings)
	}
}

func TestValidateRejectsMissingSummaryAsFirstSection(t *testing.T) {
	validation := Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "github",
		Title:    "PR",
		Body: strings.Join([]string{
			"## 변경 내용\n\n본문 계약을 새로 정의했습니다.",
			"## 요약\n\n뒤늦게 나온 요약입니다.",
			"## 확인한 것\n\ngo test로 확인했습니다.",
			"## 리뷰 포인트\n\n필수 절이 맞는지 봐 주세요.",
		}, "\n\n"),
	})
	if validation.OK {
		t.Fatalf("summary must be the first section: %+v", validation)
	}
	if !contains(validation.Critical, "summary_section_missing") {
		t.Fatalf("criticals %v missing summary_section_missing", validation.Critical)
	}
}

func TestValidateRejectsPlaceholderRequiredSection(t *testing.T) {
	validation := Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "github",
		Title:    "PR",
		Body: strings.Join([]string{
			"## 요약\n\n본문 계약을 새로 정의했습니다.",
			"## 변경 내용\n\n없음",
			"## 확인한 것\n\ngo test로 확인했습니다.",
			"## 리뷰 포인트\n\n필수 절이 맞는지 봐 주세요.",
		}, "\n\n"),
	})
	if validation.OK {
		t.Fatalf("placeholder-only required section must fail: %+v", validation)
	}
	if !contains(validation.Critical, "placeholder_section") {
		t.Fatalf("criticals %v missing placeholder_section", validation.Critical)
	}
}

func TestValidateRejectsPlanLinkAndGitLabRelatedIssuesSection(t *testing.T) {
	validation := Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateImplementationTask,
		Provider: "gitlab",
		Title:    "본문 검증",
		Body:     "## 요약\n\n본문\n\n## Related Issues\n\n#1\n\n## Plan Link\n\n/path/to/plan.md",
	})

	if validation.OK {
		t.Fatalf("validation must fail for forbidden sections: %+v", validation)
	}
	if !contains(validation.Critical, "plan_link_section_forbidden") {
		t.Fatalf("validation criticals missing plan link finding: %+v", validation)
	}
	if !contains(validation.Critical, "gitlab_related_issues_body_section_forbidden") {
		t.Fatalf("validation criticals missing gitlab related finding: %+v", validation)
	}
}

func TestValidateRejectsUnsupportedArtifactKindAndTemplate(t *testing.T) {
	validation := Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactKind("mr"),
		Template: IssueOpsTemplateFeature,
		Title:    "잘못된 artifact kind",
		Body:     "본문",
	})

	if validation.OK {
		t.Fatalf("unsupported artifact kind/template must fail: %+v", validation)
	}
	if !contains(validation.Critical, "unsupported_artifact_kind") {
		t.Fatalf("missing unsupported kind critical: %+v", validation)
	}
	if !contains(validation.Critical, "unsupported_template_for_artifact") {
		t.Fatalf("missing unsupported template critical: %+v", validation)
	}

	validation = Validate(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplateFeature,
		Title:    "PR template 조합 오류",
		Body:     "본문",
	})
	if validation.OK || !contains(validation.Critical, "unsupported_template_for_artifact") {
		t.Fatalf("PR with issue template must fail closed: %+v", validation)
	}
}

func contains(items []string, want string) bool {
	return slices.Contains(items, want)
}

func TestRenderTemplateKeepsEveryLegacyRequiredFieldKey(t *testing.T) {
	pr := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactPR,
		Template: IssueOpsTemplatePullRequest,
		Provider: "github",
		Title:    "PR",
		Fields: map[string]string{
			"intent":         "본문 계약을 요약이 먼저 오는 형태로 바꿨습니다.",
			"issue":          "Closes #513",
			"changes":        "artifacttemplate 패키지를 바꿨습니다.",
			"verification":   "go test로 확인했습니다.",
			"reviewer_focus": "별칭이 맞는지 봐 주세요.",
		},
	})
	if !pr.OK {
		t.Fatalf("legacy PR keys intent/issue/verification must still satisfy the contract: %+v", pr)
	}
	summary, _ := sectionContentFromBody(pr.Body, "요약")
	if !strings.Contains(summary, "Closes #513") {
		t.Fatalf("legacy PR issue field should fold into the summary:\n%s", pr.Body)
	}
	if verified, _ := sectionContentFromBody(pr.Body, "확인한 것"); !strings.Contains(verified, "go test") {
		t.Fatalf("legacy PR verification field should render under 확인한 것:\n%s", pr.Body)
	}

	child := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactChild,
		Template: IssueOpsTemplateChildTask,
		Provider: "github",
		Title:    "하위 작업",
		Fields: map[string]string{
			"parent_issue": "https://github.com/acme/repo/issues/1",
			"goal":         "템플릿 렌더러 구현",
			"acceptance":   "렌더러 테스트 통과",
			"non_goals":    "provider 정책 복제 제외",
			"parent_merge": "부모 브랜치에 병합된 뒤 close-children 실행",
			"cleanup":      "child 워크트리만 정리",
		},
	})
	if !child.OK {
		t.Fatalf("legacy child keys goal/parent_merge must still satisfy the contract: %+v", child)
	}
	if !contains(child.Warnings, "unrendered_field:worktree_cleanup") {
		t.Fatalf("legacy child cleanup field should surface as an unrendered warning: %v", child.Warnings)
	}

	issue := Render(IssueOpsTemplateInput{
		Kind:     IssueOpsArtifactIssue,
		Template: IssueOpsTemplateImplementationTask,
		Provider: "github",
		Title:    "이슈",
		Fields: map[string]string{
			"summary":      "요약입니다.",
			"background":   "배경입니다.",
			"acceptance":   "완료 기준입니다.",
			"scope":        "범위입니다.",
			"verification": "검증입니다.",
			"rollback":     "커밋을 되돌립니다.",
		},
	})
	if risks, ok := sectionContentFromBody(issue.Body, "위험"); !ok || !strings.Contains(risks, "커밋을 되돌립니다.") {
		t.Fatalf("issue rollback field should render under 위험:\n%s", issue.Body)
	}
}

func TestValidateIgnoresHeadingsInsideCodeFences(t *testing.T) {
	body := strings.Join([]string{
		"## 요약\n\n본문 예시를 코드 블록으로 보여 줍니다.\n\n```markdown\n## 변경 내용\n\n예시일 뿐입니다.\n```",
		"## 확인한 것\n\ngo test로 확인했습니다.",
		"## 리뷰 포인트\n\n봐 주세요.",
	}, "\n\n")
	validation := Validate(IssueOpsTemplateInput{Kind: IssueOpsArtifactPR, Template: IssueOpsTemplatePullRequest, Provider: "github", Title: "PR", Body: body})
	if !contains(validation.MissingRequiredSections, "변경 내용") {
		t.Fatalf("a heading inside a code fence must not satisfy a required section: %+v", validation)
	}

	fencedFirst := "```markdown\n## 요약\n\n예시\n```\n\n## 변경 내용\n\n바꿨습니다."
	if hasNonEmptySummarySection(fencedFirst) {
		t.Fatalf("a fenced ## 요약 must not count as the first section")
	}
}

func TestValidateRejectsListStylePlaceholder(t *testing.T) {
	body := strings.Join([]string{
		"## 요약\n\n본문 계약을 새로 정의했습니다.",
		"## 변경 내용\n\n- 없음",
		"## 확인한 것\n\ngo test로 확인했습니다.",
		"## 리뷰 포인트\n\n봐 주세요.",
	}, "\n\n")
	validation := Validate(IssueOpsTemplateInput{Kind: IssueOpsArtifactPR, Template: IssueOpsTemplatePullRequest, Provider: "github", Title: "PR", Body: body})
	if !contains(validation.Critical, "placeholder_section") {
		t.Fatalf("a required section holding only \"- 없음\" is a placeholder: %+v", validation)
	}
}

func TestInferTemplateKind(t *testing.T) {
	cases := []struct {
		kind IssueOpsArtifactKind
		body string
		want IssueOpsTemplateKind
	}{
		{IssueOpsArtifactIssue, "## 요약\n\n버그\n\n## 재현 절차\n\n1. 실행", IssueOpsTemplateBug},
		{IssueOpsArtifactIssue, "## 요약\n\n제안\n\n## 제안\n\n바꾸자", IssueOpsTemplateProposal},
		{IssueOpsArtifactIssue, "## 요약\n\n구현\n\n## 배경\n\n이유", IssueOpsTemplateImplementationTask},
		{IssueOpsArtifactIssue, "## 요약\n\n구현\n\n```\n## 재현 절차\n```", IssueOpsTemplateImplementationTask},
		{IssueOpsArtifactChild, "## 재현 절차", IssueOpsTemplateChildTask},
		{IssueOpsArtifactPR, "## 제안", IssueOpsTemplatePullRequest},
	}
	for _, tc := range cases {
		if got := InferTemplateKind(tc.kind, tc.body); got != tc.want {
			t.Fatalf("InferTemplateKind(%s, %q) = %s, want %s", tc.kind, tc.body, got, tc.want)
		}
	}
}
