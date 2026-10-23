package shelltoken

import (
	"strings"
	"unicode"
)

func SplitCommandTokens(command string) []string {
	s := &tokenSplitter{runes: []rune(command), tokens: []string{}}
	for ; s.i < len(s.runes); s.i++ {
		s.consume()
	}
	s.flush()
	return s.tokens
}

// tokenSplitter는 SplitCommandTokens의 quote 상태와 누적 토큰을 보관한다.
type tokenSplitter struct {
	runes  []rune
	i      int
	tokens []string
	// current는 지금 만드는 토큰이다.
	current strings.Builder
	quote   rune
	// started는 quote가 열렸던 토큰을 기억한다. ''/"" 같은 빈 따옴표 값은
	// 빈 문자열 argv 토큰이며, 이를 소실하면 exact flag 파싱이 뒤 토큰을 값으로
	// 오인해 명령 전체가 미분류로 떨어진다(이슈 #90 발견 3).
	started bool
}

// consume은 runes[i] 하나(필요하면 뒤따르는 escape 대상까지)를 처리한다.
func (s *tokenSplitter) consume() {
	r := s.runes[s.i]
	switch {
	case s.quote != 0:
		s.consumeQuoted(r)
	case r == '\'' || r == '"':
		s.quote = r
		s.started = true
	case r == '\\' && s.i+1 < len(s.runes):
		s.i++
		if s.runes[s.i] != '\n' && s.runes[s.i] != '\r' {
			s.current.WriteRune(s.runes[s.i])
		}
	case isShellBlank(r):
		s.flush()
	default:
		s.current.WriteRune(r)
	}
}

func (s *tokenSplitter) consumeQuoted(r rune) {
	if s.quote == '"' && r == '\\' && s.i+1 < len(s.runes) {
		s.i++
		s.current.WriteRune(doubleQuoteEscape(s.runes[s.i]))
		return
	}
	if r == s.quote {
		s.quote = 0
		return
	}
	s.current.WriteRune(r)
}

// flush는 비어 있지 않거나 quote로 시작된 토큰을 확정하고 토큰 상태를 초기화한다.
func (s *tokenSplitter) flush() {
	if s.current.Len() > 0 || s.started {
		s.tokens = append(s.tokens, s.current.String())
		s.current.Reset()
	}
	s.started = false
}

// doubleQuoteEscape은 double quote 안에서 backslash 뒤에 온 문자를 해석한다.
func doubleQuoteEscape(r rune) rune {
	switch r {
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	default:
		return r
	}
}

// HasUnquotedControlOperator는 명령을 이어 붙일 수 있는 unquoted shell
// 연산자를 보고한다.
func HasUnquotedControlOperator(command string) bool {
	var quote rune
	escaped := false
	for _, r := range command {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		switch r {
		case ';', '&', '|', '\n', '\r':
			return true
		}
	}
	return false
}

// HasShellExpansion reports a `$(...)`, `${...}`, or `$NAME` outside single
// quotes. Argv execution passes it through as characters, so a command that
// relies on the shell to expand it silently runs with the literal text.
func HasShellExpansion(command string) bool {
	var quote rune
	escaped := false
	runes := []rune(command)
	for i, r := range runes {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote == '\'' {
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			if quote == r {
				quote = 0
			} else if quote == 0 {
				quote = r
			}
			continue
		}
		if r == '$' && i+1 < len(runes) {
			next := runes[i+1]
			if next == '(' || next == '{' || next == '_' || unicode.IsLetter(next) {
				return true
			}
		}
	}
	return false
}

func isShellBlank(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// QuoteWord quotes value as one POSIX shell word using single quotes; empty
// input yields an empty quoted word.
func QuoteWord(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
