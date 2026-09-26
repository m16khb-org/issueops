package issueopsbenchmark

import "strings"

func hasAllIssueOpsConcepts(s string, concepts [][]string) bool {
	for _, variants := range concepts {
		if !containsAnyFold(s, variants...) {
			return false
		}
	}
	return true
}

func containsAllFold(s string, needles ...string) bool {
	for _, needle := range needles {
		if !ContainsFold(s, needle) {
			return false
		}
	}
	return true
}

func containsAnyFold(s string, needles ...string) bool {
	for _, needle := range needles {
		if ContainsFold(s, needle) {
			return true
		}
	}
	return false
}

func ContainsFold(s, needle string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(needle))
}

func containsHangul(s string) bool {
	for _, r := range s {
		if (r >= '가' && r <= '힣') || (r >= 'ㄱ' && r <= 'ㅎ') || (r >= 'ㅏ' && r <= 'ㅣ') {
			return true
		}
	}
	return false
}
