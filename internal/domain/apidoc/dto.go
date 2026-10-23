package apidoc

import (
	contract "issueops/internal/contract/apidoc"
	"regexp"
	"strings"
	"sync"
)

var dtoPropertyRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^\s*(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\??\s*[!:?]?\s*:`)
})
var dtoClassDeclarationRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`\b(?:export\s+)?(?:abstract\s+)?class\s+[A-Za-z_][A-Za-z0-9_]*\b`)
})

// nestDTOScanner는 NestJS DTO 클래스 본문을 줄 단위로 훑으며 데코레이터와
// 클래스 중괄호 깊이를 누적한다. 상태가 쌓이므로 포인터 리시버로만 쓴다.
type nestDTOScanner struct {
	file             string
	violations       []contract.Violation
	decorators       []string
	inClass          bool
	pendingClassBody bool
	classDepth       int
}

func CheckNestDTO(file, text string) []contract.Violation {
	s := &nestDTOScanner{file: file}
	for i, line := range strings.Split(text, "\n") {
		s.scanLine(i, line)
	}
	return s.violations
}

func (s *nestDTOScanner) scanLine(i int, line string) {
	trimmed := strings.TrimSpace(line)
	if !s.inClass {
		s.scanOutsideClass(line, trimmed)
		return
	}
	if s.classDepth != 1 {
		s.advanceDepth(line)
		return
	}
	if strings.HasPrefix(trimmed, "@") {
		s.decorators = append(s.decorators, trimmed)
		s.classDepth += BraceDepthDelta(line)
		return
	}
	m := dtoPropertyRe().FindStringSubmatch(line)
	if !isDTOPropertyLine(m, trimmed) {
		if clearsPendingDecorators(trimmed) {
			s.decorators = nil
		}
		s.advanceDepth(line)
		return
	}
	s.checkProperty(i, line, m[1])
	s.decorators = nil
	s.advanceDepth(line)
}

// scanOutsideClass는 클래스 선언과 본문 여는 중괄호를 찾는다.
func (s *nestDTOScanner) scanOutsideClass(line, trimmed string) {
	if dtoClassDeclarationRe().MatchString(trimmed) {
		s.pendingClassBody = true
	}
	if s.pendingClassBody && strings.Contains(trimmed, "{") {
		s.inClass = true
		s.pendingClassBody = false
		s.classDepth = BraceDepthDelta(line)
	}
	s.decorators = nil
}

// advanceDepth는 줄의 중괄호 변화를 반영하고, 클래스가 닫히면 상태를 초기화한다.
func (s *nestDTOScanner) advanceDepth(line string) {
	s.classDepth += BraceDepthDelta(line)
	if s.classDepth <= 0 {
		s.inClass = false
		s.decorators = nil
	}
}

// isDTOPropertyLine은 줄이 메서드나 private/static 멤버가 아닌 DTO 프로퍼티인지 판단한다.
func isDTOPropertyLine(match []string, trimmed string) bool {
	return match != nil && !strings.Contains(trimmed, "(") && !strings.HasPrefix(trimmed, "private ") && !strings.HasPrefix(trimmed, "static ")
}

// clearsPendingDecorators는 프로퍼티가 아닌 줄이 쌓인 데코레이터를 버려야 하는지 판단한다.
func clearsPendingDecorators(trimmed string) bool {
	return trimmed != "" && !strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, "}")
}

func (s *nestDTOScanner) addViolation(line int, code, message string) {
	s.violations = append(s.violations, contract.Violation{File: s.file, Line: line, Code: code, Message: message})
}

// checkProperty는 한 DTO 프로퍼티의 TypeScript 선택성과 Swagger 데코레이터를 대조한다.
func (s *nestDTOScanner) checkProperty(i int, line, name string) {
	deco := strings.Join(s.decorators, "\n")
	hasIsOptional := strings.Contains(deco, "@IsOptional")
	tsOptional := strings.Contains(line, "?:") || hasIsOptional
	swaggerOptional := strings.Contains(deco, "@ApiPropertyOptional") || apiPropertyRequiredFlag(deco, "false")
	swaggerRequiredExplicit := strings.Contains(deco, "@ApiProperty") && !strings.Contains(deco, "@ApiPropertyOptional") && (apiPropertyRequiredFlag(deco, "true") || !apiPropertyHasRequiredKey(deco))
	lineNo := i + 1
	s.checkRequiredOptionalMismatch(lineNo, name, tsOptional, swaggerOptional, swaggerRequiredExplicit)
	if !tsOptional {
		if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
			s.addViolation(lineNo, "missing_api_property", "required DTO property "+name+" is missing @ApiProperty")
		}
		return
	}
	if !swaggerOptional && !swaggerRequiredExplicit {
		s.addViolation(lineNo, "missing_api_property_optional", "optional DTO property "+name+" is missing @ApiPropertyOptional")
	}
	if !hasIsOptional {
		s.addViolation(lineNo, "missing_is_optional", "optional DTO property "+name+" is missing @IsOptional")
	}
}

// checkRequiredOptionalMismatch는 TypeScript 선택성과 Swagger 필수/선택 문서화가 어긋나는지 확인한다.
func (s *nestDTOScanner) checkRequiredOptionalMismatch(lineNo int, name string, tsOptional, swaggerOptional, swaggerRequiredExplicit bool) {
	if tsOptional && swaggerRequiredExplicit {
		s.addViolation(lineNo, "required_optional_mismatch", "optional DTO property "+name+" is documented as required (@ApiProperty required: true/default) but is optional in TypeScript/validation")
	} else if !tsOptional && swaggerOptional {
		s.addViolation(lineNo, "required_optional_mismatch", "required DTO property "+name+" is documented as optional in Swagger but is required in TypeScript/validation")
	}
}

func BraceDepthDelta(line string) int {
	delta := 0
	for _, ch := range line {
		switch ch {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

var apiPropertyObjectValueRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`required\s*:\s*(true|false)`) })

// apiPropertyRequiredFlag reports whether a @ApiProperty(...) decorator object
// sets the required key to the given boolean. Plain string search stays
// decorator-safe because other decorators on the same property do not carry a
// required key.
func apiPropertyRequiredFlag(deco, want string) bool {
	if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
		return false
	}
	for _, m := range apiPropertyObjectValueRe().FindAllStringSubmatch(deco, -1) {
		if m[1] == want {
			return true
		}
	}
	return false
}

func apiPropertyHasRequiredKey(deco string) bool {
	if !strings.Contains(deco, "@ApiProperty") || strings.Contains(deco, "@ApiPropertyOptional") {
		return false
	}
	return apiPropertyObjectValueRe().MatchString(deco)
}
