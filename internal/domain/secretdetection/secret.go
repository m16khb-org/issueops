package secretdetection

import (
	"regexp"
	"strings"
	"sync"
)

type pattern struct {
	kind string
	re   *regexp.Regexp
}

var patterns = sync.OnceValue(func() []pattern {
	return []pattern{
		{"credential assignment", regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token|password|passwd|credential|private[_-]?key)\s*[:=]\s*\S+`)},
		{"private key block", regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`)},
		{"GitHub/GitLab token", regexp.MustCompile(`(?i)(ghp|gho|ghu|ghs|ghr|glpat|gldt|glft)_[A-Za-z0-9_]{20,}`)},
		{"JWT", regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`)},
	}
})

// Match locates the earliest secret-like value without carrying the value:
// Line is 1-based and Kind names the pattern that matched.
type Match struct {
	Line int
	Kind string
}

func Contains(value string) bool {
	for _, p := range patterns() {
		if p.re.MatchString(value) {
			return true
		}
	}
	return false
}

// Find returns the earliest secret-like match in value.
func Find(value string) (Match, bool) {
	offset, kind := -1, ""
	for _, p := range patterns() {
		if loc := p.re.FindStringIndex(value); loc != nil && (offset < 0 || loc[0] < offset) {
			offset, kind = loc[0], p.kind
		}
	}
	if offset < 0 {
		return Match{}, false
	}
	return Match{Line: strings.Count(value[:offset], "\n") + 1, Kind: kind}, true
}
