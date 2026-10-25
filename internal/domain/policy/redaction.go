package policy

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var secretPathRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(^|/)(\.env(\.|$)|id_rsa|id_dsa|id_ecdsa|id_ed25519|.*\.pem$|.*\.key$|.*\.p12$|.*\.pfx$|.*credentials.*|.*secret.*)`)
})
var secretArgRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)((token|password|passwd|secret|api[_-]?key|credential|authorization)=|authorization[[:space:]]*:[[:space:]]*bearer[[:space:]]+[^[:space:]]+)`)
})
var knownTokenRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(gh[pousr]_[a-z0-9]{20,}|github_pat_[a-z0-9_]{20,}|xox[baprs]-[a-z0-9-]{20,}|sk-[a-z0-9]{20,}|AKIA[0-9A-Z]{16})`)
})

var diagnosticURLPattern = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`https?://[^\s]+`) })

const maxBoundedDiagnosticBytes = 4096

func CleanEnvAllowlist(items []string) []string {
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func ValidEnvName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if !validEnvNameRune(r, i == 0) {
			return false
		}
	}
	return true
}

// validEnvNameRune accepts ASCII letters and '_'; digits are allowed only after the first rune.
func validEnvNameRune(r rune, first bool) bool {
	switch {
	case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		return true
	case r >= '0' && r <= '9':
		return !first
	}
	return false
}

func RedactArgv(argv []string) []string {
	out := make([]string, len(argv))
	for i, arg := range argv {
		out[i] = RedactFreeform(arg)
	}
	return out
}

func RedactFreeform(s string) string {
	if SecretLikeArg(s) {
		return "<redacted>"
	}
	return s
}

func RedactDiagnostic(s string) string {
	if redacted := RedactFreeform(s); redacted != s {
		return redacted
	}
	return diagnosticURLPattern().ReplaceAllString(s, "[REDACTED_URL]")
}

func BoundedDiagnostic(value string, limit int) string {
	if limit <= 0 || limit > maxBoundedDiagnosticBytes {
		limit = maxBoundedDiagnosticBytes
	}
	value = strings.TrimSpace(RedactDiagnostic(value))
	if len(value) > limit {
		value = TruncateBytes(value, limit) + "...[truncated]"
	}
	return value
}

func SecretLikeArg(arg string) bool {
	return secretArgRe().MatchString(arg) || knownTokenRe().MatchString(arg) || secretPathRe().MatchString(filepath.ToSlash(arg))
}

// SecretLikeBytes is SecretLikeArg without converting the bytes to a string,
// for scanning a stream chunk by chunk. The regexes cost about 150 ms per MB,
// so ASCII text without any keyword they require skips them.
func SecretLikeBytes(value []byte) bool {
	if !mayHoldSecret(value) {
		return false
	}
	if filepath.Separator != '/' {
		return SecretLikeArg(string(value))
	}
	return secretArgRe().Match(value) || knownTokenRe().Match(value) || secretPathRe().Match(value)
}

// secretKeywords holds, lowercased, a literal every alternative of
// secretArgRe, knownTokenRe and secretPathRe requires. Keep it in step with
// those patterns (TestSecretKeywordsCoverEveryPattern).
var secretKeywords = [][]byte{
	[]byte("token"), []byte("passw"), []byte("secret"), []byte("api"), []byte("credential"), []byte("authorization"),
	[]byte("ghp_"), []byte("gho_"), []byte("ghu_"), []byte("ghs_"), []byte("ghr_"), []byte("github_pat_"),
	[]byte("xox"), []byte("sk-"), []byte("akia"),
	[]byte(".env"), []byte("id_rsa"), []byte("id_dsa"), []byte("id_ecdsa"), []byte("id_ed25519"),
	[]byte(".pem"), []byte(".key"), []byte(".p12"), []byte(".pfx"),
}

// mayHoldSecret reports whether value could match a secret pattern: it holds a
// non-ASCII byte ((?i) also folds non-ASCII letters such as the Kelvin sign
// onto k, so those always take the regex path) or, ASCII-lowercased, one of
// secretKeywords. It allocates nothing.
func mayHoldSecret(value []byte) bool {
	const block, overlap = 4096, 32
	var buf [block + overlap]byte
	for start := 0; start < len(value); start += block {
		end := min(start+block+overlap, len(value))
		n := 0
		for _, c := range value[start:end] {
			if c >= 0x80 {
				return true
			}
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			buf[n] = c
			n++
		}
		for _, keyword := range secretKeywords {
			if bytes.Contains(buf[:n], keyword) {
				return true
			}
		}
	}
	return false
}
