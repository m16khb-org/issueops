// Package artifactreadability judges whether an issue, PR, or progress-report
// body is written for a human reader: enough Korean, no leaked hashes or
// local paths, no placeholder sections. It is a pure function over text; it
// does no file or network I/O and never re-implements the structural
// judgment (required sections, summary-first) that internal/domain/
// artifacttemplate already owns.
package artifactreadability

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"issueops/internal/domain/artifacttemplate"
	"issueops/internal/domain/issueopsbodysync"
)

// Kind identifies what is being checked. Issue, Child, and PR bodies are
// validated against artifacttemplate's structural contract in addition to
// this package's own rules. Completion has no template: it is the free-form
// progress-report draft written for `reflect-completion --body-file`, so it
// skips structural checks but raises local_path and commit_sha_full to
// critical and enforces a length budget (devil's-advocate memo B).
type Kind string

const (
	KindIssue      Kind = "issue"
	KindChild      Kind = "child"
	KindPR         Kind = "pr"
	KindCompletion Kind = "completion"
)

// completionMaxRunes bounds the progress-report draft so an agent cannot
// paste a plan or spec in whole. Chosen by the plan (2,000 characters).
const completionMaxRunes = 2000

// summaryMaxRunes flags a summary section that has grown past a skimmable
// length (warning, not critical).
const summaryMaxRunes = 400

// minHangulChars and maxEnglishRatio match the Python gate this package
// replaces (skills/issueops-remote-write/scripts/remote_artifact_gate.py).
const (
	minHangulChars  = 20
	maxEnglishRatio = 1.2
)

type Input struct {
	Kind     Kind                                  `json:"kind"`
	Template artifacttemplate.IssueOpsTemplateKind `json:"template,omitempty"`
	Title    string                                `json:"title"`
	Body     string                                `json:"body"`
	// Fields is optional. When set, unrendered legacy `--field` keys surface
	// as warnings the same way artifacttemplate.Validate reports them, so a
	// single readability response covers both structural and field-level
	// feedback.
	Fields map[string]string `json:"fields,omitempty"`
}

type Finding struct {
	Code    string `json:"code"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

type Report struct {
	OK       bool      `json:"ok"`
	Critical []Finding `json:"critical"`
	Warnings []Finding `json:"warnings"`
}

var harnessTerms = []string{
	"generation", "lease", "fingerprint", "봉인", "sealed", "canonical worktree",
	"grill", "plan-prep", "readback", "reconcile", "brooks", "turing", "shannon", "boehm",
}

// harnessTermRes match each harness term as a whole word, so "release" never
// reads as "lease". Korean terms have no ASCII boundary and match as-is.
var harnessTermRes = func() []*regexp.Regexp {
	res := make([]*regexp.Regexp, 0, len(harnessTerms))
	for _, term := range harnessTerms {
		pattern := regexp.QuoteMeta(term)
		if isASCII(term) {
			pattern = `\b` + pattern + `\b`
		}
		res = append(res, regexp.MustCompile(`(?i)`+pattern))
	}
	return res
}()

var slopHedgePatterns = []string{"고 할 수 있습니다", "것으로 보입니다"}
var slopIntroPatterns = []string{"살펴보겠습니다", "하고자 합니다"}

var (
	codeFenceRe  = regexp.MustCompile("(?s)```.*?```")
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
	urlRe        = regexp.MustCompile(`https?://\S+`)
	pathLikeRe   = regexp.MustCompile(`(?:^|\s)[./~]?[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+`)
	localPathRe  = regexp.MustCompile(`(?:/Users/|/home/)[^\s\x60]*`)
	resultOnlyRe = regexp.MustCompile(`(?i)^\s*[-*]?\s*(pass|통과|ok)\.?\s*$`)
)

// Check judges body against Kind's rules and, for Issue/Child/PR, the
// matching artifacttemplate contract. It never mutates input and performs no
// I/O. Line numbers in findings refer to lines of input.Body.
func Check(input Input) Report {
	report := Report{Critical: []Finding{}, Warnings: []Finding{}}
	// prose is the authored body with harness-managed regions blanked out;
	// code additionally blanks fenced and inline code. Both keep every line
	// break so a finding's line is the line in the original body.
	prose := blankManagedRegions(input.Body)
	code := blankCode(prose)
	template := artifacttemplate.IssueOpsArtifactKind("")

	if input.Kind != KindCompletion {
		template = templateKindFor(input.Kind)
		structural := artifacttemplate.Validate(artifacttemplate.IssueOpsTemplateInput{
			Kind:     template,
			Template: input.Template,
			Title:    input.Title,
			Body:     prose,
			Fields:   input.Fields,
		})
		if slices.Contains(structural.Critical, "summary_section_missing") {
			report.Critical = append(report.Critical, Finding{Code: "summary_section_missing", Message: "첫 절이 ## 요약이고 비어 있지 않아야 합니다."})
		}
		if slices.Contains(structural.Critical, "required_section_missing") {
			report.Critical = append(report.Critical, Finding{Code: "required_section_missing", Message: "필수 절이 빠졌습니다: " + strings.Join(structural.MissingRequiredSections, ", ")})
		}
		if slices.Contains(structural.Critical, "placeholder_section") {
			report.Critical = append(report.Critical, Finding{Code: "placeholder_section", Message: "필수 절에 자리 표시만 있습니다."})
		}
		for _, w := range structural.Warnings {
			if key, ok := strings.CutPrefix(w, "unrendered_field:"); ok {
				report.Warnings = append(report.Warnings, Finding{Code: w, Message: "본문에 렌더하지 않는 필드입니다: " + key})
			}
		}
	}

	hangul, englishWords := scoreLanguage(input.Title + "\n" + prose)
	if hangul < minHangulChars {
		report.Critical = append(report.Critical, Finding{Code: "korean_ratio", Message: "한글이 최소 20자 이상이어야 합니다."})
	} else if float64(englishWords)/float64(hangul) > maxEnglishRatio {
		report.Critical = append(report.Critical, Finding{Code: "korean_ratio", Message: "영어 단어 비율이 한글 대비 1.2를 넘습니다."})
	}

	// local_path and commit_sha_full block a progress-report draft (memo B)
	// but only warn on issue and PR bodies.
	pathOrSHA := func(f Finding) {
		if input.Kind == KindCompletion {
			report.Critical = append(report.Critical, f)
		} else {
			report.Warnings = append(report.Warnings, f)
		}
	}
	for i, line := range strings.Split(code, "\n") {
		lineNo := i + 1
		hasSHA256, hasCommitSHA := hexWords(line)
		if hasSHA256 {
			report.Critical = append(report.Critical, Finding{Code: "sha256_hex", Line: lineNo, Message: "코드 밖에 64자리 hex가 있습니다."})
		}
		if (strings.Contains(line, "/Users/") || strings.Contains(line, "/home/")) && localPathRe.MatchString(line) {
			pathOrSHA(Finding{Code: "local_path", Line: lineNo, Message: "로컬 절대 경로가 있습니다."})
		}
		if hasCommitSHA {
			pathOrSHA(Finding{Code: "commit_sha_full", Line: lineNo, Message: "코드 밖에 40자리 커밋 SHA 전문이 있습니다."})
		}
		lower := strings.ToLower(line)
		for j, term := range harnessTerms {
			if strings.Contains(lower, term) && harnessTermRes[j].MatchString(line) {
				report.Warnings = append(report.Warnings, Finding{Code: "harness_term", Line: lineNo, Message: "하네스 용어가 나옵니다: " + term})
			}
		}
	}

	if input.Kind != KindCompletion {
		if content, ok := artifacttemplate.SummarySection(prose); ok && utf8.RuneCountInString(content) > summaryMaxRunes {
			report.Warnings = append(report.Warnings, Finding{Code: "summary_too_long", Message: "요약이 400자를 넘습니다."})
		}
		for _, title := range artifacttemplate.OptionalSectionTitles(template, input.Template) {
			if content, ok := artifacttemplate.SectionContent(prose, title); ok && content == "" {
				report.Warnings = append(report.Warnings, Finding{Code: "empty_optional_section", Message: "선택 절이 비어 있습니다: " + title})
			}
		}
	}

	for _, verifyTitle := range []string{"확인한 것", "검증"} {
		if content, ok := artifacttemplate.SectionContent(prose, verifyTitle); ok && slices.ContainsFunc(strings.Split(content, "\n"), resultOnlyRe.MatchString) {
			report.Warnings = append(report.Warnings, Finding{Code: "result_only_pass", Message: verifyTitle + " 절에 결과만 있는 줄이 있습니다."})
		}
	}

	if hedgeOrIntroCount(code) > 0 || strings.Count(code, "—") >= 3 || strings.Count(code, "→") >= 3 {
		report.Warnings = append(report.Warnings, Finding{Code: "slop_pattern", Message: "fluent-korean이 잡는 AI 작문 패턴이 있습니다."})
	}

	if firstDuplicateSentence(code) != "" {
		report.Warnings = append(report.Warnings, Finding{Code: "duplicate_sentence", Message: "같은 문장이 두 절 이상에 반복됩니다."})
	}

	if input.Kind == KindCompletion && utf8.RuneCountInString(strings.TrimSpace(prose)) > completionMaxRunes {
		report.Critical = append(report.Critical, Finding{Code: "result_too_long", Message: "진행 결과 원고가 2,000자를 넘습니다."})
	}

	report.Critical = sortFindings(report.Critical)
	report.Warnings = sortFindings(report.Warnings)
	report.OK = len(report.Critical) == 0
	return report
}

var (
	maskHashRe      = regexp.MustCompile(`\b(?:[0-9a-fA-F]{64}|[0-9a-fA-F]{40})\b`)
	maskLocalPathRe = regexp.MustCompile(`(?:/Users/|/home/)[^\s\x60)]*`)
)

// MaskHarnessValues hides full hashes (64 and 40 hex digits) and local
// absolute paths in text the harness renders for human readers, such as plan
// review findings, and says what was left out.
func MaskHarnessValues(text string) string {
	text = maskHashRe.ReplaceAllString(text, "[해시 생략]")
	return maskLocalPathRe.ReplaceAllString(text, "[로컬 경로 생략]")
}

// KindFor maps a template artifact kind to the readability kind that
// judges it.
func KindFor(kind artifacttemplate.IssueOpsArtifactKind) Kind {
	switch kind {
	case artifacttemplate.IssueOpsArtifactChild:
		return KindChild
	case artifacttemplate.IssueOpsArtifactPR:
		return KindPR
	default:
		return KindIssue
	}
}

// RefusalError explains a refused publication finding by finding, so the
// author can fix the body without running the preview again.
func RefusalError(report Report) error {
	items := make([]string, 0, len(report.Critical))
	for _, f := range report.Critical {
		item := f.Code
		if f.Line > 0 {
			item += fmt.Sprintf(" (line %d)", f.Line)
		}
		items = append(items, item+": "+f.Message)
	}
	return fmt.Errorf("readability check failed; fix the body and preview again: %s", strings.Join(items, "; "))
}

func templateKindFor(kind Kind) artifacttemplate.IssueOpsArtifactKind {
	switch kind {
	case KindChild:
		return artifacttemplate.IssueOpsArtifactChild
	case KindPR:
		return artifacttemplate.IssueOpsArtifactPR
	default:
		return artifacttemplate.IssueOpsArtifactIssue
	}
}

// blankManagedRegions removes harness-authored blocks (progress report,
// plan review, issue-create marker) before checking authored prose, so the
// harness's own rendering is never judged as if the author wrote it.
func blankManagedRegions(body string) string {
	out := body
	for _, region := range issueopsbodysync.ManagedRegions(body) {
		out = strings.Replace(out, region.Block, strings.Repeat("\n", strings.Count(region.Block, "\n")), 1)
	}
	return out
}

// blankCode replaces fenced and inline code with spaces, keeping line breaks.
func blankCode(text string) string {
	blank := func(match string) string {
		return strings.Map(func(r rune) rune {
			if r == '\n' {
				return r
			}
			return ' '
		}, match)
	}
	text = codeFenceRe.ReplaceAllStringFunc(text, blank)
	return inlineCodeRe.ReplaceAllStringFunc(text, blank)
}

// scoreLanguage counts Hangul syllables and English words in prose the same
// way the Python gate it replaces did: code, URLs, and paths are removed
// first, and an English word only counts when Unicode word boundaries
// surround it, so "PR을" is not an English word.
func scoreLanguage(text string) (hangul int, englishWords int) {
	prose := codeFenceRe.ReplaceAllString(text, " ")
	prose = inlineCodeRe.ReplaceAllString(prose, " ")
	prose = urlRe.ReplaceAllString(prose, " ")
	prose = pathLikeRe.ReplaceAllString(prose, " ")
	runes := []rune(prose)
	for _, r := range runes {
		if r >= '가' && r <= '힣' {
			hangul++
		}
	}
	return hangul, countEnglishWords(runes)
}

// countEnglishWords mirrors Python's re.findall(r"\b[A-Za-z][A-Za-z0-9_+-]*\b")
// with Unicode-aware \b, including its backtracking to the last boundary
// inside a greedy match.
func countEnglishWords(runes []rune) int {
	boundary := func(i int) bool {
		before := i > 0 && isWordRune(runes[i-1])
		after := i < len(runes) && isWordRune(runes[i])
		return before != after
	}
	count := 0
	for i := 0; i < len(runes); {
		if !isASCIILetter(runes[i]) || !boundary(i) {
			i++
			continue
		}
		end := i + 1
		for end < len(runes) && isASCIIWordTail(runes[end]) {
			end++
		}
		for end > i && !boundary(end) {
			end--
		}
		if end == i {
			i++
			continue
		}
		count++
		i = end
	}
	return count
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isASCIILetter(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIWordTail(r rune) bool {
	return isASCIILetter(r) || (r >= '0' && r <= '9') || r == '_' || r == '+' || r == '-'
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func hedgeOrIntroCount(body string) int {
	count := 0
	for _, p := range slopHedgePatterns {
		count += strings.Count(body, p)
	}
	for _, p := range slopIntroPatterns {
		count += strings.Count(body, p)
	}
	return count
}

// firstDuplicateSentence reports the first sentence that appears in more
// than one `## ` section.
func firstDuplicateSentence(body string) string {
	seen := map[string]int{}
	for _, sec := range strings.Split(body, "\n## ") {
		local := map[string]bool{}
		for _, s := range splitSentences(sec) {
			s = strings.TrimSpace(s)
			if utf8.RuneCountInString(s) < 8 || local[s] {
				continue
			}
			local[s] = true
			seen[s]++
			if seen[s] > 1 {
				return s
			}
		}
	}
	return ""
}

// splitSentences splits at line breaks and at ".", "!", or "?" followed by
// whitespace.
func splitSentences(text string) []string {
	var out []string
	start := 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\n':
			out = append(out, text[start:i])
			start = i + 1
		case '.', '!', '?':
			j := i + 1
			for j < len(text) && (text[j] == ' ' || text[j] == '\t' || text[j] == '\r') {
				j++
			}
			if j > i+1 || (j < len(text) && text[j] == '\n') {
				out = append(out, text[start:i])
				start = j
				i = j - 1
			}
		}
	}
	return append(out, text[start:])
}

// hexWords reports whether line holds an ASCII word that is exactly 64 or
// exactly 40 hex digits: the same test as \b[0-9a-fA-F]{64}\b and
// \b[0-9a-fA-F]{40}\b, without a regexp pass per line.
func hexWords(line string) (sha256, commit bool) {
	for i := 0; i < len(line); {
		if !isASCIIWordByte(line[i]) {
			i++
			continue
		}
		start, allHex := i, true
		for i < len(line) && isASCIIWordByte(line[i]) {
			allHex = allHex && isHexByte(line[i])
			i++
		}
		if allHex {
			sha256 = sha256 || i-start == 64
			commit = commit || i-start == 40
		}
	}
	return sha256, commit
}

func isASCIIWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isHexByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func sortFindings(findings []Finding) []Finding {
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Code != findings[j].Code {
			return findings[i].Code < findings[j].Code
		}
		return findings[i].Line < findings[j].Line
	})
	return findings
}
