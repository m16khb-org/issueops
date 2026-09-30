package issueops

import "regexp"

var (
	maskHashRe      = regexp.MustCompile(`\b(?:[0-9a-fA-F]{64}|[0-9a-fA-F]{40})\b`)
	maskLocalPathRe = regexp.MustCompile(`(?:/Users/|/home/)[^\s\x60)]*`)
)

// maskHarnessValues hides full hashes (64 and 40 hex digits) and local
// absolute paths in text the harness renders for human readers, such as plan
// review findings, and says what was left out.
func maskHarnessValues(text string) string {
	text = maskHashRe.ReplaceAllString(text, "[해시 생략]")
	return maskLocalPathRe.ReplaceAllString(text, "[로컬 경로 생략]")
}

var planReviewVerdictLabels = map[string]string{"pass": "통과", "revise": "수정 요청", "stop": "중단"}

// planReviewVerdictLabel is the word a reader sees for a plan-review verdict.
func planReviewVerdictLabel(verdict string) string {
	if label := planReviewVerdictLabels[verdict]; label != "" {
		return label
	}
	return verdict
}
