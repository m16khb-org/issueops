package apidoc

import (
	contract "issueops/internal/contract/apidoc"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var nestRouteRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`@(Get|Post|Put|Patch|Delete|All|Head|Options)\s*\(\s*(?:["'\x60]([^"'\x60]*)["'\x60])?`)
})
var nestPathParamRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`:([A-Za-z0-9_]+)`) })
var apiResponseStatusRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`@ApiResponses?\s*\(`) })
var apiResponseStatusValueRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`status\s*:\s*(?:HttpStatus\.)?([0-9]+|[A-Z_]+)`) })

var nestHttpStatusNames = map[string]int{
	"OK":                    200,
	"CREATED":               201,
	"ACCEPTED":              202,
	"NO_CONTENT":            204,
	"BAD_REQUEST":           400,
	"UNAUTHORIZED":          401,
	"PAYMENT_REQUIRED":      402,
	"FORBIDDEN":             403,
	"NOT_FOUND":             404,
	"CONFLICT":              409,
	"GONE":                  410,
	"PRECONDITION_FAILED":   412,
	"UNPROCESSABLE_ENTITY":  422,
	"TOO_MANY_REQUESTS":     429,
	"INTERNAL_SERVER_ERROR": 500,
	"NOT_IMPLEMENTED":       501,
	"BAD_GATEWAY":           502,
	"SERVICE_UNAVAILABLE":   503,
	"GATEWAY_TIMEOUT":       504,
}
var methodNameRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^\s*(?:async\s+)?[A-Za-z_][A-Za-z0-9_]*\s*\(`) })
var queryNamedRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`@Query\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`) })
var bodyNamedRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`@Body\s*\(\s*["'\x60]([^"'\x60]+)["'\x60]`) })

func CheckNestController(file, text string) []contract.Violation {
	lines := strings.Split(text, "\n")
	var violations []contract.Violation
	for i := range lines {
		if !methodNameRe().MatchString(lines[i]) {
			continue
		}
		start := controllerDecoratorStart(lines, i)
		end := i
		for end < len(lines) && !strings.Contains(lines[end], "{") {
			end++
		}
		block := strings.Join(lines[start:min(end+1, len(lines))], "\n")
		route := nestRouteRe().FindStringSubmatch(block)
		if route == nil {
			continue
		}
		violations = append(violations, nestRouteViolations(file, i+1, block, route[2], text)...)
	}
	return violations
}

// nestRouteViolations은 한 라우트 블록에 대한 문서화 위반을 검사 순서대로 모은다.
func nestRouteViolations(file string, line int, block, routePath, text string) []contract.Violation {
	var violations []contract.Violation
	add := func(code, message string) {
		violations = append(violations, contract.Violation{File: file, Line: line, Code: code, Message: message})
	}
	if code, message, ok := apiOperationViolation(block); ok {
		add(code, message)
	}
	for _, m := range nestPathParamRe().FindAllStringSubmatch(routePath, -1) {
		if !hasNamedDecorator(block, "ApiParam", m[1]) {
			add("missing_api_param", "path parameter :"+m[1]+" is missing @ApiParam documentation")
		}
	}
	if strings.Contains(block, "@Headers") && !strings.Contains(block, "@ApiHeader") {
		add("missing_api_header", "@Headers usage is missing @ApiHeader documentation")
	}
	for _, m := range queryNamedRe().FindAllStringSubmatch(block, -1) {
		if !hasNamedDecorator(block, "ApiQuery", m[1]) {
			add("missing_api_query", "named query parameter "+m[1]+" is missing @ApiQuery documentation")
		}
	}
	violations = append(violations, nestBodyAndResponseViolations(file, line, block, text)...)
	return violations
}

// nestBodyAndResponseViolations는 @Body 문서화와 400/401 응답 위반을 검사한다.
func nestBodyAndResponseViolations(file string, line int, block, text string) []contract.Violation {
	var violations []contract.Violation
	add := func(code, message string) {
		violations = append(violations, contract.Violation{File: file, Line: line, Code: code, Message: message})
	}
	if bodyNamedRe().MatchString(block) && !strings.Contains(block, "@ApiBody") {
		add("missing_api_body", "named @Body usage is missing @ApiBody documentation")
	}
	if (strings.Contains(block, "@Body") || strings.Contains(block, "@Query") || strings.Contains(block, "@Headers")) && !HasNestResponseStatus(block, 400) {
		add("missing_400_response", "body/query/header validation surface is missing a 400 Swagger response")
	}
	if IsNestPrivateRoute(block, text) && !HasNestResponseStatus(block, 401) {
		add("missing_401_response", "private/auth route is missing a 401 Swagger response")
	}
	return violations
}

// apiOperationViolation은 @ApiOperation 관련 첫 번째 위반만 반환한다.
func apiOperationViolation(block string) (code, message string, found bool) {
	switch {
	case !strings.Contains(block, "@ApiOperation"):
		return "missing_api_operation", "route method is missing @ApiOperation", true
	case !strings.Contains(block, "description"):
		return "missing_api_operation_description", "@ApiOperation is missing description", true
	case !strings.Contains(block, "### "):
		return "invalid_api_operation_description_format", "@ApiOperation.description must use the project sectioned Markdown format", true
	}
	return "", "", false
}

// hasNamedDecorator는 block에 @<decorator>({ name: '<name>' }) 형태가 있는지 확인한다.
func hasNamedDecorator(block, decorator, name string) bool {
	return regexp.MustCompile(`@` + decorator + `\s*\(\s*\{[^}]*name\s*:\s*["'\x60]` + regexp.QuoteMeta(name) + `["'\x60]`).MatchString(block)
}

// controllerDecoratorStart returns the first line of the decorator block for
// the member whose signature sits at lines[member]. It walks upward across
// decorators and their multi-line object interiors (summary:/description:
// properties inside @ApiOperation({...})) using paren/brace balance, and stops
// at member or class boundaries. A naive line-prefix walk stops inside
// multi-line decorator objects and silently skips the whole route method.
func controllerDecoratorStart(lines []string, member int) int {
	start := member
	balance := 0
	for start > 0 {
		line := lines[start-1]
		trimmed := strings.TrimSpace(line)
		delta := BraceDepthDelta(line) + parenDepthDelta(line)
		if strings.Contains(trimmed, "class ") && strings.HasSuffix(trimmed, "{") {
			break
		}
		if trimmed == "}" || strings.HasSuffix(trimmed, ";") {
			break
		}
		if balance != 0 || delta < 0 || strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, ".") || trimmed == "" {
			balance += delta
			start--
			continue
		}
		break
	}
	return start
}

func parenDepthDelta(line string) int {
	delta := 0
	for _, ch := range line {
		switch ch {
		case '(':
			delta++
		case ')':
			delta--
		}
	}
	return delta
}

func HasNestResponseStatus(block string, status int) bool {
	if status == 400 && strings.Contains(block, "@ApiBadRequestResponse") {
		return true
	}
	if status == 401 && strings.Contains(block, "@ApiUnauthorizedResponse") {
		return true
	}
	for _, span := range apiResponseDecoratorSpans(block) {
		for _, m := range apiResponseStatusValueRe().FindAllStringSubmatch(span, -1) {
			code, err := strconv.Atoi(m[1])
			if err != nil {
				code = nestHttpStatusNames[m[1]]
			}
			if code == status {
				return true
			}
		}
	}
	return false
}

// apiResponseDecoratorSpans returns the balanced argument text of every
// @ApiResponse / @ApiResponses occurrence, so both object form
// ({ status: 404 }) and array form ([{ status: 404 }, ...]) are scanned.
func apiResponseDecoratorSpans(block string) []string {
	var spans []string
	for _, loc := range apiResponseStatusRe().FindAllStringIndex(block, -1) {
		depth := 0
		started := false
		for i := loc[1] - 1; i < len(block); i++ {
			switch block[i] {
			case '(':
				depth++
				started = true
			case ')':
				depth--
			}
			if started && depth <= 0 {
				spans = append(spans, block[loc[1]:i])
				break
			}
		}
	}
	return spans
}

func IsNestPrivateRoute(block, fileText string) bool {
	if strings.Contains(block, "@Public") {
		return false
	}
	return strings.Contains(block, "@ApiBearerAuth") || strings.Contains(fileText, "@ApiBearerAuth") || strings.Contains(block, "UseGuards") || strings.Contains(block, "RequireTier") || hasClassLevelGuard(fileText)
}

// hasClassLevelGuard reports whether the file declares a controller-level auth
// guard (@UseGuards/@ApiBearerAuth/RequireTier above the class declaration),
// which makes every member route private unless it opts out with @Public.
func hasClassLevelGuard(fileText string) bool {
	before, _, ok := strings.Cut(fileText, "class ")
	if !ok {
		return false
	}
	header := before
	return strings.Contains(header, "@UseGuards") || strings.Contains(header, "@ApiBearerAuth") || strings.Contains(header, "RequireTier")
}
