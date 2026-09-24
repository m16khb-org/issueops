package artifactreadability

import (
	"strings"
	"testing"
	"time"

	"issueops/internal/domain/artifacttemplate"
)

func validPRBody() string {
	return strings.Join([]string{
		"## 요약\n\n이슈와 PR 본문을 사람이 읽는 문서로 바꾸는 가독성 검사를 추가합니다. 게시 명령 안에서 항상 실행됩니다.",
		"## 변경 내용\n\n새 패키지 artifactreadability를 추가하고 규칙마다 fixture 테스트를 뒀습니다.",
		"## 확인한 것\n\ngo test로 규칙마다 통과와 실패 fixture를 확인했습니다.",
		"## 리뷰 포인트\n\n규칙 임계값이 적절한지 봐 주세요.",
	}, "\n\n")
}

func TestCheckPassesCleanPRBody(t *testing.T) {
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: validPRBody()})
	if !report.OK {
		t.Fatalf("clean PR body should pass: %+v", report)
	}
}

func TestCheckSHA256HexIsCriticalWithLine(t *testing.T) {
	body := validPRBody() + "\n\n## 남은 일\n\n다음 커밋 해시로 확인합니다 e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	if report.OK {
		t.Fatalf("body with a bare SHA-256 must fail: %+v", report)
	}
	found := false
	for _, c := range report.Critical {
		if c.Code == "sha256_hex" {
			found = true
			if c.Line == 0 {
				t.Fatalf("sha256_hex finding should carry a line number: %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("criticals missing sha256_hex: %+v", report.Critical)
	}
}

func TestCheckAllowsHashInsideCodeFence(t *testing.T) {
	body := validPRBody() + "\n\n## 남은 일\n\n```\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n```"
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	for _, c := range report.Critical {
		if c.Code == "sha256_hex" {
			t.Fatalf("hash inside a code fence must not trigger sha256_hex: %+v", report.Critical)
		}
	}
}

func TestCheckKoreanRatioCritical(t *testing.T) {
	body := strings.Join([]string{
		"## Summary\n\nThis body is written mostly in English prose with almost no Korean at all here.",
		"## Changes\n\nAdded a new readability package with English-only prose across every section body.",
		"## Verified\n\nRan go test locally and confirmed every rule fixture passes as expected in English.",
		"## Review Points\n\nPlease check whether the English ratio threshold is calibrated correctly for reviewers.",
	}, "\n\n")
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "readability check", Body: body})
	if report.OK {
		t.Fatalf("english-only body must fail korean_ratio: %+v", report)
	}
	found := false
	for _, c := range report.Critical {
		if c.Code == "korean_ratio" {
			found = true
		}
	}
	if !found {
		t.Fatalf("criticals missing korean_ratio: %+v", report.Critical)
	}
}

func TestCheckDelegatesStructuralFindingsToArtifactTemplate(t *testing.T) {
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "PR", Body: "## 변경 내용\n\n요약이 없는 본문입니다.\n\n## 확인한 것\n\n확인했습니다.\n\n## 리뷰 포인트\n\n봐 주세요."})
	if report.OK {
		t.Fatalf("body missing summary-first must fail: %+v", report)
	}
	found := false
	for _, c := range report.Critical {
		if c.Code == "summary_section_missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("criticals missing summary_section_missing: %+v", report.Critical)
	}
}

func TestCheckLocalPathAndCommitSHAAreCriticalOnlyForCompletion(t *testing.T) {
	prBody := validPRBody() + "\n\n## 남은 일\n\n로그는 /Users/dev/workspace/issueops 아래 있고 커밋 abcd1234abcd1234abcd1234abcd1234abcd1234로 확인했습니다."
	prReport := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: prBody})
	if !prReport.OK {
		t.Fatalf("local path / commit sha are warnings for PR bodies, not critical: %+v", prReport)
	}
	hasWarning := func(code string) bool {
		for _, w := range prReport.Warnings {
			if w.Code == code {
				return true
			}
		}
		return false
	}
	if !hasWarning("local_path") || !hasWarning("commit_sha_full") {
		t.Fatalf("PR warnings missing local_path/commit_sha_full: %+v", prReport.Warnings)
	}

	completionBody := "두 이슈를 서로 다른 세션에서 동시에 진행해도 간섭하지 않음을 실제로 확인했습니다.\n\n- 로컬 경로: /Users/dev/workspace/issueops\n- 커밋: abcd1234abcd1234abcd1234abcd1234abcd1234"
	completionReport := Check(Input{Kind: KindCompletion, Title: "진행 결과", Body: completionBody})
	if completionReport.OK {
		t.Fatalf("completion draft with local path and commit sha must fail: %+v", completionReport)
	}
	criticalCode := func(code string) bool {
		for _, c := range completionReport.Critical {
			if c.Code == code {
				return true
			}
		}
		return false
	}
	if !criticalCode("local_path") || !criticalCode("commit_sha_full") {
		t.Fatalf("completion criticals missing local_path/commit_sha_full: %+v", completionReport.Critical)
	}
}

func TestCheckResultTooLongOnlyAppliesToCompletion(t *testing.T) {
	long := strings.Repeat("완료 보고 문장입니다. ", 250)
	report := Check(Input{Kind: KindCompletion, Title: "진행 결과", Body: long})
	if report.OK {
		t.Fatalf("completion draft over 2000 runes must fail: %+v", report)
	}
	found := false
	for _, c := range report.Critical {
		if c.Code == "result_too_long" {
			found = true
		}
	}
	if !found {
		t.Fatalf("criticals missing result_too_long: %+v", report.Critical)
	}
}

func TestCheckUnrenderedFieldWarning(t *testing.T) {
	report := Check(Input{
		Kind:     KindPR,
		Template: artifacttemplate.IssueOpsTemplatePullRequest,
		Title:    "가독성 검사 추가",
		Body:     validPRBody(),
		Fields:   map[string]string{"worktree_cleanup": "워크트리 상태 확인"},
	})
	if !report.OK {
		t.Fatalf("unrendered field is a warning, not a failure: %+v", report)
	}
	found := false
	for _, w := range report.Warnings {
		if w.Code == "unrendered_field:worktree_cleanup" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings missing unrendered_field:worktree_cleanup: %+v", report.Warnings)
	}
}

func TestCheckHarnessTermIsWarningOnly(t *testing.T) {
	body := validPRBody() + "\n\n## 남은 일\n\ngeneration과 lease는 그대로 유지합니다."
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	if !report.OK {
		t.Fatalf("harness terms must warn, not fail: %+v", report)
	}
	found := false
	for _, w := range report.Warnings {
		if w.Code == "harness_term" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings missing harness_term: %+v", report.Warnings)
	}
}

func TestCheckManagedRegionsAreExcludedFromChecks(t *testing.T) {
	body := validPRBody() + "\n\n<!-- issueops:completion:start -->\n## 진행 결과\n\n커밋 abcd1234abcd1234abcd1234abcd1234abcd1234로 확인했습니다.\n<!-- issueops:completion:end -->"
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	if !report.OK {
		t.Fatalf("managed completion region must not be checked against author rules: %+v", report)
	}
}

func TestCheckWarnsOnSummaryTooLongEmptyOptionalSectionAndResultOnlyPass(t *testing.T) {
	longSummary := strings.Repeat("이 문장은 요약을 지나치게 길게 만들기 위한 반복 문장입니다. ", 30)
	body := strings.Join([]string{
		"## 요약\n\n" + longSummary,
		"## 변경 내용\n\n새 패키지를 추가했습니다.",
		"## 확인한 것\n\n통과",
		"## 리뷰 포인트\n\n봐 주세요.",
		"## 대안과 선택 이유\n\n",
	}, "\n\n")
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	if !report.OK {
		t.Fatalf("these are warnings, not criticals: %+v", report)
	}
	wantCodes := map[string]bool{"summary_too_long": false, "empty_optional_section": false, "result_only_pass": false}
	for _, w := range report.Warnings {
		if _, ok := wantCodes[w.Code]; ok {
			wantCodes[w.Code] = true
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Fatalf("warnings missing %q: %+v", code, report.Warnings)
		}
	}
}

func TestCheckWarnsOnSlopPatternAndDuplicateSentence(t *testing.T) {
	repeated := "가독성 검사는 게시 명령 안에서 항상 실행됩니다."
	body := strings.Join([]string{
		"## 요약\n\n이 변경은 가독성 검사를 추가한다고 할 수 있습니다. " + repeated,
		"## 변경 내용\n\n" + repeated,
		"## 확인한 것\n\ngo test로 확인했습니다.",
		"## 리뷰 포인트\n\n봐 주세요.",
	}, "\n\n")
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	if !report.OK {
		t.Fatalf("slop pattern and duplicate sentence are warnings, not criticals: %+v", report)
	}
	wantCodes := map[string]bool{"slop_pattern": false, "duplicate_sentence": false}
	for _, w := range report.Warnings {
		if _, ok := wantCodes[w.Code]; ok {
			wantCodes[w.Code] = true
		}
	}
	for code, found := range wantCodes {
		if !found {
			t.Fatalf("warnings missing %q: %+v", code, report.Warnings)
		}
	}
}

func TestCheckRejectsRequiredSectionMissingAndPlaceholderSection(t *testing.T) {
	missing := Check(Input{Kind: KindChild, Template: artifacttemplate.IssueOpsTemplateChildTask, Title: "하위 작업", Body: "## 요약\n\n부모 이슈의 템플릿 렌더러 구현을 맡습니다."})
	if missing.OK {
		t.Fatalf("child body missing required sections must fail: %+v", missing)
	}
	found := false
	for _, c := range missing.Critical {
		if c.Code == "required_section_missing" {
			found = true
		}
	}
	if !found {
		t.Fatalf("criticals missing required_section_missing: %+v", missing.Critical)
	}

	placeholder := Check(Input{Kind: KindChild, Template: artifacttemplate.IssueOpsTemplateChildTask, Title: "하위 작업", Body: strings.Join([]string{
		"## 요약\n\n부모 이슈의 템플릿 렌더러 구현을 맡습니다.",
		"## 완료 기준\n\n없음",
		"## 범위\n\n렌더러 패키지만 바꿉니다.",
		"## 선행 조건과 병합 조건\n\n부모 브랜치 병합 뒤 close-children을 실행합니다.",
	}, "\n\n")})
	if placeholder.OK {
		t.Fatalf("placeholder-only required section must fail: %+v", placeholder)
	}
	found = false
	for _, c := range placeholder.Critical {
		if c.Code == "placeholder_section" {
			found = true
		}
	}
	if !found {
		t.Fatalf("criticals missing placeholder_section: %+v", placeholder.Critical)
	}
}

func TestCheckLargeBodyIsFast(t *testing.T) {
	body := validPRBody() + "\n\n## 남은 일\n\n" + strings.Repeat("성능 측정을 위한 문장입니다. ", 2000)
	if len(body) < 50*1024 {
		t.Fatalf("fixture body should be around 50KB, got %d bytes", len(body))
	}
	start := time.Now()
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "성능 측정", Body: body})
	elapsed := time.Since(start)
	if elapsed > 200*time.Millisecond {
		t.Fatalf("50KB body check took %s, want well under 200ms (no network/file I/O, O(n) scan)", elapsed)
	}
	if !report.OK {
		t.Fatalf("large repetitive body should still pass readability: %+v", report)
	}
}

func TestCheckLineNumbersSurviveCodeFencesAndManagedRegions(t *testing.T) {
	body := strings.Join([]string{
		"<!-- issueops:issue-create:0123456789abcdef0123456789abcdef -->",
		validPRBody(),
		"## 남은 일",
		"",
		"```",
		"첫 줄",
		"둘째 줄",
		"```",
		"",
		"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
	}, "\n")
	wantLine := len(strings.Split(body, "\n"))
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	for _, c := range report.Critical {
		if c.Code == "sha256_hex" {
			if c.Line != wantLine {
				t.Fatalf("sha256_hex line = %d, want %d (line in the original body)", c.Line, wantLine)
			}
			return
		}
	}
	t.Fatalf("criticals missing sha256_hex: %+v", report.Critical)
}

func TestCheckHarnessTermMatchesWholeWordsOnly(t *testing.T) {
	body := validPRBody() + "\n\n## 남은 일\n\n다음 release 노트에 정리합니다. please 같은 단어도 그대로 둡니다."
	report := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: body})
	for _, w := range report.Warnings {
		if w.Code == "harness_term" {
			t.Fatalf("release/please must not match the harness term lease: %+v", w)
		}
	}
}

// TestScoreLanguageMatchesPythonGate pins the counts the deleted Python gate
// (skills/issueops-remote-write/scripts/remote_artifact_gate.py) produced for
// the same text. Python's \b is Unicode-aware, so an English word glued to a
// Korean particle ("PR을") is not counted as English.
func TestScoreLanguageMatchesPythonGate(t *testing.T) {
	cases := []struct {
		name          string
		text          string
		hangul, words int
	}{
		{"particles", "PR을 만들고 lease가 풀리면 issueops가 본문을 다시 읽습니다. Go 코드와 CLI 응답을 확인했습니다.", 30, 2},
		{"mixed", "이 변경은 `go test ./...`와 https://example.com/a/b 링크, internal/domain/x 경로를 씁니다. The check runs inside remote commands now 그리고 결과를 보여 줍니다.", 24, 7},
		{"english", "## 요약\n\nThis body is mostly English prose with a few words 한글 조금.\n\n```\n한글이 코드 안에만 많이 있습니다 한글 한글 한글\n```", 6, 10},
	}
	for _, tc := range cases {
		hangul, words := scoreLanguage(tc.text)
		if hangul != tc.hangul || words != tc.words {
			t.Fatalf("%s: scoreLanguage = (%d, %d), Python gate = (%d, %d)", tc.name, hangul, words, tc.hangul, tc.words)
		}
	}
}

// 진행 결과 구간에는 해시·커밋 SHA·로컬 경로가 없어야 한다(intent 성공 기준 4).
// 원고에서는 code span이나 코드 블록으로 감싸도 그대로 렌더되므로 예외가 없다.
func TestCompletionDraftRefusesHarnessValuesInsideCode(t *testing.T) {
	sha := strings.Repeat("ab", 20)
	body := "두 이슈를 서로 다른 세션에서 동시에 진행해도 간섭하지 않음을 확인했습니다.\n\n- 커밋: `" + sha + "`\n\n```\n/Users/dev/wt\n" + strings.Repeat("cd", 32) + "\n```\n"
	report := Check(Input{Kind: KindCompletion, Body: body})
	for _, code := range []string{"commit_sha_full", "local_path", "sha256_hex"} {
		if !hasCriticalCode(report, code) {
			t.Fatalf("a completion draft must refuse %s even inside code: %+v", code, report.Critical)
		}
	}
	pr := Check(Input{Kind: KindPR, Template: artifacttemplate.IssueOpsTemplatePullRequest, Title: "가독성 검사 추가", Body: validPRBody() + "\n\n## 남은 일\n\n```\n" + sha + "\n```"})
	if !pr.OK {
		t.Fatalf("issue and PR bodies keep the code exception: %+v", pr)
	}
}

func hasCriticalCode(report Report, code string) bool {
	for _, f := range report.Critical {
		if f.Code == code {
			return true
		}
	}
	return false
}
