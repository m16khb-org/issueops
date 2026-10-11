package remote

import (
	"strings"
	"unicode"
)

func issueOpsRemoteTokens(text string) map[string]bool {
	tokens := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_')
	}) {
		token = strings.Trim(token, "-_")
		if len(token) < 3 || issueOpsRemoteStopWords[token] {
			continue
		}
		tokens[token] = true
	}
	return tokens
}

var issueOpsRemoteStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "this": true, "that": true, "from": true,
	"문제": true, "현재": true, "근거": true, "완료": true, "기준": true, "검증": true, "비목표": true,
}

func issueOpsRemoteOverlap(left, right map[string]bool) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	intersect := 0
	for token := range left {
		if right[token] {
			intersect++
		}
	}
	return float64(intersect) / float64(min(len(left), len(right)))
}

// issueOpsRemoteLabelKeywords는 라벨 이름별 휴리스틱 키워드와 일치 시 점수를 정의한다.
var issueOpsRemoteLabelKeywords = map[string]struct {
	tokens []string
	score  float64
}{
	"enhancement":   {[]string{"feature", "request", "개선", "추가", "지원"}, 1},
	"bug":           {[]string{"bug", "defect", "failure", "오류", "결함"}, 1},
	"documentation": {[]string{"docs", "documentation", "문서", "skill", "prompt"}, 0.75},
}

func issueOpsRemoteLabelHeuristic(issueTokens map[string]bool, candidate IssueOpsRemoteLabelCandidate) float64 {
	name := strings.ToLower(strings.TrimSpace(candidate.Name))
	rule, ok := issueOpsRemoteLabelKeywords[name]
	if !ok {
		return 0
	}
	for _, token := range rule.tokens {
		if issueTokens[token] {
			return rule.score
		}
	}
	return 0
}
