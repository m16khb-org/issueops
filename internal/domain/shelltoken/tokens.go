package shelltoken

import "strings"

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

// HasActiveShellSpecialQuoting은 Bash/zsh ANSI-C $'...'와 Bash locale $"..."
// quoting을 보고한다. 이 구문은 보호 대상 실행 파일 이름을 합성할 수 있어
// supervised shell 명령에서는 거부된다. quote되거나 escape된 달러 기호는
// 리터럴 데이터로 남는다.
func HasActiveShellSpecialQuoting(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	for i, r := range runes {
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
		if r == '$' && i+1 < len(runes) && (runes[i+1] == '\'' || runes[i+1] == '"') {
			return true
		}
	}
	return false
}

// HasActiveZshEqualsExpansion은 zsh의 unquoted 단어 선두 =name command-path
// expansion과 =(...) 임시 파일 process substitution을 보고한다. quote되거나
// backslash로 escape된 등호와 평범한 NAME=value shell 대입은 이 검사에서
// 리터럴로 남는다.
func HasActiveZshEqualsExpansion(command string) bool {
	scanner := equalsScanner{runes: []rune(command), wordStart: true}
	for i, r := range scanner.runes {
		if scanner.active(i, r) {
			return true
		}
	}
	return false
}

// equalsScanner는 HasActiveZshEqualsExpansion의 quote/escape 상태와 현재
// 위치가 단어 선두인지 여부를 보관한다.
type equalsScanner struct {
	runes     []rune
	quote     rune
	escaped   bool
	wordStart bool
}

// active는 runes[i]=r 하나를 처리하고 활성 =expansion이면 true를 돌려준다.
func (s *equalsScanner) active(i int, r rune) bool {
	if s.escaped {
		s.escaped = false
		s.wordStart = false
		return false
	}
	if r == '\\' && s.quote != '\'' {
		s.escaped = true
		return false
	}
	if s.quoteTransition(r) {
		return false
	}
	if isShellBlank(r) {
		s.wordStart = true
		return false
	}
	if s.wordStart && r == '=' && i+1 < len(s.runes) {
		return true
	}
	s.wordStart = false
	return false
}

// quoteTransition은 r이 quote 안의 문자이거나 quote를 열고 닫을 때 이를
// 반영하고 true를 돌려준다. quote 처리된 문자는 단어 시작이 아니다.
func (s *equalsScanner) quoteTransition(r rune) bool {
	switch {
	case s.quote != 0:
		if r == s.quote {
			s.quote = 0
		}
	case r == '\'' || r == '"':
		s.quote = r
	default:
		return false
	}
	s.wordStart = false
	return true
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

// HasActiveCommandSubstitution은 shell이 실행하게 될 backtick, $(...),
// unquoted process substitution을 보고한다. single quote되거나 명시적으로
// escape된 형태는 리터럴이다. double quote는 command substitution을
// 유지하지만 Bash와 zsh에서 <(...)와 >(...)는 리터럴 데이터로 만든다.
func HasActiveCommandSubstitution(command string) bool {
	scanner := substitutionScanner{runes: []rune(command)}
	for i, r := range scanner.runes {
		if scanner.active(i, r) {
			return true
		}
	}
	return false
}

// substitutionScanner는 HasActiveCommandSubstitution의 quote/escape 상태를
// 보관한다.
type substitutionScanner struct {
	runes   []rune
	quote   rune
	escaped bool
}

// active는 runes[i]=r 하나를 처리하고 활성 command substitution이면 true를
// 돌려준다.
func (s *substitutionScanner) active(i int, r rune) bool {
	if s.escaped {
		s.escaped = false
		return false
	}
	if r == '\\' && s.quote != '\'' {
		s.escaped = true
		return false
	}
	if s.quoteTransition(r) {
		return false
	}
	return r == '`' || s.substitutionOpenerAt(i, r)
}

// quoteTransition은 r이 quote 상태를 바꾸거나 single quote 안의 문자일 때 이를
// 반영하고 true를 돌려준다.
func (s *substitutionScanner) quoteTransition(r rune) bool {
	switch {
	case s.quote == '\'':
		if r == '\'' {
			s.quote = 0
		}
	case r == '\'':
		s.quote = '\''
	case r == '"':
		if s.quote == '"' {
			s.quote = 0
		} else {
			s.quote = '"'
		}
	default:
		return false
	}
	return true
}

// substitutionOpenerAt은 runes[i]=r 뒤에 '('가 이어지는 $( 또는 unquoted
// <( / >( 인지 판단한다.
func (s *substitutionScanner) substitutionOpenerAt(i int, r rune) bool {
	if i+1 >= len(s.runes) || s.runes[i+1] != '(' {
		return false
	}
	return r == '$' || s.quote == 0 && (r == '<' || r == '>')
}

// HasActiveOutputRedirect는 quote되거나 escape된 리터럴 데이터 밖의 shell
// output redirection을 보고한다. unquoted '>'는 fd 접두, append,
// stdout/stderr 결합 형태를 포함해 전부 활성으로 본다.
func HasActiveOutputRedirect(command string) bool {
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
		if quote == '\'' {
			if r == '\'' {
				quote = 0
			}
			continue
		}
		if quote == '"' {
			if r == '"' {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '>' {
			return true
		}
	}
	return false
}

// HasActiveParameterOrTildeExpansion은 겉보기에 상대 경로인 operand를 환경이
// 제어하는 경로로 바꿀 수 있는 shell expansion을 보고한다. POSIX single
// quote와 backslash escape는 리터럴로 남지만, parameter expansion은 double
// quote 안에서도 여전히 활성이다.
func HasActiveParameterOrTildeExpansion(command string) bool {
	scanner := expansionScanner{runes: []rune(command), wordStart: true}
	for i, r := range scanner.runes {
		if scanner.active(i, r) {
			return true
		}
	}
	return false
}

// expansionScanner는 HasActiveParameterOrTildeExpansion의 quote/escape 상태와
// 현재 단어의 시작 위치를 보관한다.
type expansionScanner struct {
	runes       []rune
	quote       rune
	escaped     bool
	wordStart   bool
	wordStartAt int
}

// active는 runes[i]=r 하나를 처리하고 활성 expansion이면 true를 돌려준다.
func (s *expansionScanner) active(i int, r rune) bool {
	if s.escaped {
		s.escaped = false
		s.wordStart = false
		return false
	}
	if r == '\\' && s.quote != '\'' {
		s.escaped = true
		return false
	}
	if s.quoteTransition(r) {
		return false
	}
	if s.parameterExpansionAt(i, r) || s.tildeExpansionAt(i, r) {
		return true
	}
	s.trackWordBoundary(i, r)
	return false
}

// quoteTransition은 r이 quote 상태를 바꾸거나 single quote 안의 문자일 때 이를
// 반영하고 true를 돌려준다. quote 처리된 문자는 단어 시작이 아니다.
func (s *expansionScanner) quoteTransition(r rune) bool {
	switch {
	case s.quote == '\'':
		if r == '\'' {
			s.quote = 0
		}
	case r == '\'':
		s.quote = '\''
	case r == '"':
		if s.quote == '"' {
			s.quote = 0
		} else {
			s.quote = '"'
		}
	default:
		return false
	}
	s.wordStart = false
	return true
}

func (s *expansionScanner) parameterExpansionAt(i int, r rune) bool {
	return r == '$' && i+1 < len(s.runes) && shellParameterStart(s.runes[i+1])
}

func (s *expansionScanner) tildeExpansionAt(i int, r rune) bool {
	return s.quote == 0 && r == '~' && (s.wordStart || shellAssignmentTildeStart(s.runes[s.wordStartAt:i]))
}

func (s *expansionScanner) trackWordBoundary(i int, r rune) {
	if s.quote == 0 && isShellBlank(r) {
		s.wordStart = true
		s.wordStartAt = i + 1
	} else {
		s.wordStart = false
	}
}

func shellAssignmentTildeStart(prefix []rune) bool {
	value := string(prefix)
	equals := strings.IndexRune(value, '=')
	if equals <= 0 || !shellAssignmentName(value[:equals]) {
		return false
	}
	assignmentValue := value[equals+1:]
	return assignmentValue == "" || strings.HasSuffix(assignmentValue, ":")
}

func shellAssignmentName(value string) bool {
	for i, r := range value {
		if i == 0 {
			if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') {
				return false
			}
			continue
		}
		if r != '_' && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return value != ""
}

// HasActivePathnameExpansion은 unquoted shell glob 또는 brace expansion을
// 보고한다. quote되거나 backslash로 escape된 문법은 리터럴 데이터이므로
// 계속 허용된다.
func HasActivePathnameExpansion(command string) bool {
	runes := []rune(command)
	var quote rune
	escaped := false
	for i, r := range runes {
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
		case '*', '?':
			return true
		case '[':
			if unquotedExpansionCloser(runes, i+1, ']') {
				return true
			}
		case '{':
			if unquotedBraceExpansion(runes, i+1) {
				return true
			}
		}
	}
	return false
}

func unquotedExpansionCloser(runes []rune, start int, closer rune) bool {
	var quote rune
	escaped := false
	content := 0
	for i := start; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			escaped = false
			content++
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				content++
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == closer {
			return content > 0
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return false
		}
		content++
	}
	return false
}

// braceScanner는 unquotedBraceExpansion의 quote/escape 상태와 구분자 발견
// 여부를 보관한다.
type braceScanner struct {
	quote       rune
	escaped     bool
	delimiter   bool
	previousDot bool
}

// noteLiteral은 r을 brace 본문 문자로 기록한다. ".."는 구분자이며, 이는 quote되거나
// escape된 문자에도 그대로 적용된다.
func (s *braceScanner) noteLiteral(r rune) {
	if r == '.' && s.previousDot {
		s.delimiter = true
	}
	s.previousDot = r == '.'
}

// step은 r 하나를 처리한다. done이 true이면 result가 최종 판정이다.
func (s *braceScanner) step(r rune) (result, done bool) {
	switch {
	case s.escaped:
		s.escaped = false
		s.noteLiteral(r)
	case r == '\\' && s.quote != '\'':
		s.escaped = true
	case s.quote != 0:
		if r == s.quote {
			s.quote = 0
		} else {
			s.noteLiteral(r)
		}
	case r == '\'' || r == '"':
		s.quote = r
	case r == '}':
		return s.delimiter, true
	case isShellBlank(r):
		return false, true
	default:
		if r == ',' {
			s.delimiter = true
		}
		s.noteLiteral(r)
	}
	return false, false
}

func unquotedBraceExpansion(runes []rune, start int) bool {
	var scanner braceScanner
	for i := start; i < len(runes); i++ {
		if result, done := scanner.step(runes[i]); done {
			return result
		}
	}
	return false
}

func isShellBlank(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func shellParameterStart(r rune) bool {
	return r == '{' || r == '(' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("*@#?-$!", r)
}

// QuoteWord quotes value as one POSIX shell word using single quotes; empty
// input yields an empty quoted word.
func QuoteWord(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
