package remote

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
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

// issueOpsRemoteLabelKeywords는 키워드 규칙이 있는 라벨 이름별 유형 근거 키워드다.
// 한글 키워드는 토큰의 접두와 맞으면(조사·어미가 붙은 토큰 포함), 영어 키워드는 토큰 전체가 같을 때 맞는다.
var issueOpsRemoteLabelKeywords = map[string][]string{
	"bug": {
		"bug", "defect", "failure", "fail", "failed", "fails", "error", "crash", "regression", "broken",
		"오류", "결함", "버그", "실패", "회귀", "깨진", "깨지", "잘못", "누락", "오동작", "재현", "레이스", "교착", "손상", "유실",
		"막힌", "막혀", "거부되", "고친다", "수정한다",
	},
	"enhancement": {
		"feature", "request", "support", "add", "enhancement",
		"개선", "추가", "지원", "도입", "확장", "새로운", "신규", "기능", "강화",
	},
	"documentation": {"docs", "documentation", "readme", "문서"},
}

// issueOpsRemoteMinLabelEvidence는 규칙 라벨을 고르기 위한 전체 유형 근거 토큰의 최소 개수다.
// 근거 하나로 고른 경우의 정답률이 둘 이상일 때보다 낮아 그 미만이면 기권한다.
const issueOpsRemoteMinLabelEvidence = 2

// issueOpsRemoteTypeEvidence는 이슈 토큰에서 규칙 라벨별로 센 유형 근거다.
type issueOpsRemoteTypeEvidence struct {
	matched map[string][]string // 규칙 라벨 이름 -> 맞은 토큰(정렬)
	total   int                 // 모든 규칙 라벨의 맞은 토큰 수 합
}

// issueOpsRemoteKeywordMatches는 토큰이 키워드에 맞는지 판정한다.
func issueOpsRemoteKeywordMatches(token, keyword string) bool {
	if keyword[0] < utf8.RuneSelf {
		return token == keyword
	}
	return strings.HasPrefix(token, keyword)
}

// issueOpsRemoteLabelEvidence는 이슈 토큰마다 규칙 라벨별로 최대 한 번씩 근거를 센다.
func issueOpsRemoteLabelEvidence(issueTokens map[string]bool) issueOpsRemoteTypeEvidence {
	evidence := issueOpsRemoteTypeEvidence{matched: map[string][]string{}}
	for label, keywords := range issueOpsRemoteLabelKeywords {
		var matched []string
		for token := range issueTokens {
			for _, keyword := range keywords {
				if issueOpsRemoteKeywordMatches(token, keyword) {
					matched = append(matched, token)
					break
				}
			}
		}
		sort.Strings(matched)
		evidence.matched[label] = matched
		evidence.total += len(matched)
	}
	return evidence
}

// issueOpsRemoteLabelEvidenceScore는 규칙 라벨의 점수를 돌려준다. 이름에 규칙이 없으면 ok가 false다.
// 전체 근거가 최소 개수 미만이면 0이다.
func issueOpsRemoteLabelEvidenceScore(evidence issueOpsRemoteTypeEvidence, candidate IssueOpsRemoteLabelCandidate) (score float64, matched []string, ok bool) {
	matched, ok = evidence.matched[issueOpsRemoteRuleLabel(candidate.Name)]
	if !ok {
		return 0, nil, false
	}
	if evidence.total < issueOpsRemoteMinLabelEvidence {
		return 0, matched, true
	}
	return float64(len(matched)) / float64(evidence.total), matched, true
}

// issueOpsRemoteRuleSynonyms maps the type word of common label schemes to
// the keyword rule it scores with.
var issueOpsRemoteRuleSynonyms = map[string]string{
	"bug": "bug", "enhancement": "enhancement", "feature": "enhancement",
	"documentation": "documentation", "docs": "documentation",
}

// issueOpsRemoteRuleLabel resolves a label name to its keyword rule. QA: a
// repository labelled kind/feature, kind/bug scored every label 0 because
// rules matched only the bare GitHub default names. The type word is the
// part after the last "/" or ":" (kind/feature, type: bug, docs).
func issueOpsRemoteRuleLabel(name string) string {
	word := strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndexAny(word, "/:"); i >= 0 {
		word = strings.TrimSpace(word[i+1:])
	}
	return issueOpsRemoteRuleSynonyms[word]
}
