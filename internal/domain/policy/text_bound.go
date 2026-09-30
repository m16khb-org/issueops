package policy

import "unicode/utf8"

// TruncateBytes returns the longest prefix of s that fits in maxBytes bytes
// without ending inside a UTF-8 sequence. When s is valid UTF-8, so is the result.
func TruncateBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for i := 0; i < utf8.UTFMax-1 && cut > 0 && !utf8.RuneStart(s[cut]); i++ {
		cut--
	}
	return s[:cut]
}

// TailBytes returns the longest suffix of s that fits in maxBytes bytes
// without starting inside a UTF-8 sequence. When s is valid UTF-8, so is the result.
func TailBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	start := len(s) - maxBytes
	for i := 0; i < utf8.UTFMax-1 && start < len(s) && !utf8.RuneStart(s[start]); i++ {
		start++
	}
	return s[start:]
}

// TruncateRunes returns the first maxRunes characters of s.
func TruncateRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i]
		}
		n++
	}
	return s
}

// TrimIncompleteRune drops a trailing UTF-8 sequence that was cut short, such
// as the tail a byte-limited writer leaves when it stops mid-character.
func TrimIncompleteRune(s string) string {
	for back := 1; back < utf8.UTFMax && back <= len(s); back++ {
		start := len(s) - back
		if utf8.RuneStart(s[start]) {
			if !utf8.FullRuneInString(s[start:]) {
				return s[:start]
			}
			return s
		}
	}
	return s
}
