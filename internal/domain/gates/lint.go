package gates

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	model "issueops/internal/contract/gates"
)

// LintCandidate는 lint할 게이트 하나와 그 출처다. Problems는 spec을 원장으로 옮기는
// 단계에서 이미 난 오류다(이때 Gate는 비어 있을 수 있다).
type LintCandidate struct {
	Source   string
	Gate     Gate
	Problems []string
}

var (
	// planSpecLineRe는 계획 문서 안의 gates init spec 한 줄이다. 목록 기호, 번호,
	// `--gate "..."`, 한 쌍의 따옴표·백틱으로 감싼 줄을 허용한다(감싸기는 unwrapSpec이 벗긴다).
	planSpecLineRe = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^\s*(?:[-*]\s+|\d+\.\s+)?(?:--gate\s+)?((["'` + "`" + `]?)G\d+[A-Za-z0-9.\-]*\s*:.*\|\s*CHECK:.*?)\s*\\?\s*$`)
	})
	checkMentionRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`\bCHECK:`) })
	// gateIDLineRe는 목록·번호·표 행이 게이트 ID로 시작하는 줄이다(`- G1: ...`, `| G2 | ...`).
	gateIDLineRe = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^\s*(?:[-*]\s+|\d+\.\s+|\|\s*)` + "`?" + `G\d+[A-Za-z0-9.\-]*\s*[:|]`)
	})
)

// LintSpecCandidate는 gates init과 같은 방식으로 spec 하나를 원장 게이트로 옮긴다.
func LintSpecCandidate(source, spec string) LintCandidate {
	rendered, err := renderGateSpec(spec)
	if err != nil {
		return LintCandidate{Source: source, Gate: Gate{ID: specID(spec)}, Problems: []string{err.Error()}}
	}
	gates := Parse(rendered).Gates
	if len(gates) == 0 {
		return LintCandidate{Source: source, Gate: Gate{ID: specID(spec)}, Problems: []string{fmt.Sprintf("gate spec %q did not produce a gate", spec)}}
	}
	return LintCandidate{Source: source, Gate: gates[0]}
}

// PlanLintCandidates는 계획 문서에서 게이트를 찾는다. 한 줄짜리 spec과 원장 형식
// (`- [ ] G1: ...` 아래 들여쓴 CHECK/EXPECT)을 모두 읽는다. 둘 다 아닌데 `CHECK:`가
// 들어 있거나 게이트 ID로 시작하는 줄 번호는 unparsed로 돌려준다. 그런 게이트는
// 검사되지 않았다는 뜻이다.
func PlanLintCandidates(file, text string) (candidates []LintCandidate, unparsed []int) {
	lines := strings.Split(text, "\n")
	consumed := map[int]bool{}
	for i, line := range lines {
		if m := planSpecLineRe().FindStringSubmatch(line); m != nil {
			spec := strings.ReplaceAll(unwrapSpec(m[1], m[2]), `\"`, `"`)
			candidates = append(candidates, LintSpecCandidate(file+":"+strconv.Itoa(i+1), spec))
			consumed[i] = true
		}
	}
	for _, gate := range Parse(text).Gates {
		consumed[gate.CheckboxLine] = true
		if strings.TrimSpace(gate.CheckCmd) == "" {
			continue
		}
		candidates = append(candidates, LintCandidate{Source: file + ":" + strconv.Itoa(gate.CheckboxLine+1), Gate: gate})
	}
	for i, line := range lines {
		if !consumed[i] && !attrLineRe().MatchString(line) && (checkMentionRe().MatchString(line) || gateIDLineRe().MatchString(line)) {
			unparsed = append(unparsed, i+1)
		}
	}
	return candidates, unparsed
}

// LedgerLintCandidates는 기존 원장 파일의 게이트를 돌려준다. ABANDON된 게이트는
// 실행되지 않으므로 뺀다.
func LedgerLintCandidates(file, text string) []LintCandidate {
	var candidates []LintCandidate
	for _, gate := range Parse(text).Gates {
		if gate.Abandoned {
			continue
		}
		candidates = append(candidates, LintCandidate{Source: file, Gate: gate})
	}
	return candidates
}

// LintStatic는 실행이나 정책 평가 없이 판정할 수 있는 문제를 찾는다. argv가 nil이면
// CHECK를 argv로 옮기지 못한 것이므로 정책 평가를 건너뛴다.
func LintStatic(candidate LintCandidate) (result model.LintGate, argv []string) {
	gate := candidate.Gate
	result = model.LintGate{Source: candidate.Source, ID: gate.ID, Check: gate.CheckCmd, Expect: gate.Expect,
		Problems: append([]string(nil), candidate.Problems...)}
	if len(result.Problems) > 0 {
		return result, nil
	}
	if strings.TrimSpace(gate.CheckCmd) == "" {
		result.Problems = append(result.Problems, "no CHECK: gates check never runs this gate, so it stays unchecked")
		return result, nil
	}
	argv, checkError := CheckArgv(gate.CheckCmd)
	if checkError != "" {
		result.Problems = append(result.Problems, checkError)
		return result, nil
	}
	result.Argv = argv
	if strings.Contains(gate.CheckCmd, "`") {
		result.Problems = append(result.Problems, "CHECK contains markdown backticks; argv keeps them as characters (see argv), so the command cannot run as intended. Write the bare command")
	}
	if consumed := strings.Count(gate.CheckCmd, `\`) - strings.Count(strings.Join(argv, " "), `\`); consumed > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"the argv tokenizer consumed %d backslash(es): inside double quotes \\X becomes X and \\n a newline; use single quotes to keep a regex or escape as written (see argv)", consumed))
	}
	expect := strings.TrimSpace(gate.Expect)
	if len(expect) >= 2 && strings.HasPrefix(expect, "`") && strings.HasSuffix(expect, "`") {
		result.Problems = append(result.Problems, "EXPECT is wrapped in markdown backticks, which are matched literally, so it never matches command output. Write the bare text")
	}
	switch _, isRegex, err := expectRegex(expect); {
	case isRegex && err != nil:
		result.Problems = append(result.Problems, fmt.Sprintf("EXPECT regex does not compile (RE2), so it never matches: %v", err))
	case expect == "":
		result.Warnings = append(result.Warnings, "no EXPECT: exit code 0 alone marks this gate met")
	case !isRegex && strings.HasPrefix(expect, "/"):
		result.Warnings = append(result.Warnings, "EXPECT starts with / but is not /pattern/flags, so it is matched as a literal line")
	case !isRegex && len(expect) > 2 && strings.HasPrefix(expect, "^") && strings.HasSuffix(expect, "$"):
		result.Problems = append(result.Problems, fmt.Sprintf("EXPECT %s is an anchored regex but is matched as a literal line, so it never matches; write /%s/", expect, expect))
	case !isRegex && literalLooksLikeRegex(expect):
		result.Warnings = append(result.Warnings, fmt.Sprintf("EXPECT %s looks like a regex but is matched as a literal line; write /pattern/ if a regex is meant", expect))
	}
	return result, argv
}

// literalLooksLikeRegex reports regex syntax that a literal EXPECT keeps as
// characters: an anchor at either end or an escape/wildcard sequence.
func literalLooksLikeRegex(expect string) bool {
	if strings.HasPrefix(expect, "^") || (strings.HasSuffix(expect, "$") && !strings.HasSuffix(expect, `\$`)) {
		return true
	}
	for _, token := range []string{`\.`, `\d`, `\s`, `\w`, `.*`, `.+`} {
		if strings.Contains(expect, token) {
			return true
		}
	}
	return false
}

// DeniedLint는 command policy가 거부한 CHECK를 문제로 기록한다.
func DeniedLint(result model.LintGate, denyReasons []string) model.LintGate {
	result.PolicyDenied = true
	result.Problems = append(result.Problems, "check denied by policy: "+strings.Join(denyReasons, "; "))
	return result
}

// SummarizeLint는 게이트별 결과를 응답으로 묶는다. unparsed는 읽지 못한 계획 줄 위치다.
func SummarizeLint(gates []model.LintGate, unparsed []string, warnings []string) model.LintResult {
	result := model.LintResult{OK: true, SchemaVersion: model.SchemaVersion, Gates: gates, UnparsedLines: unparsed,
		Warnings: warnings, TotalGates: len(gates)}
	for _, gate := range gates {
		if len(gate.Problems) > 0 {
			result.GatesWithProblems++
		}
	}
	result.Clean = result.GatesWithProblems == 0 && len(unparsed) == 0
	return result
}

// UnparsedWarning은 게이트처럼 보이지만 읽지 못한 계획 줄을 알린다.
func UnparsedWarning(file string, lines []int) string {
	parts := make([]string, len(lines))
	for i, line := range lines {
		parts[i] = strconv.Itoa(line)
	}
	return fmt.Sprintf("%s: lines %s look like gates (a gate id or CHECK:) but are neither a one-line gate spec (\"G1: outcome | CHECK: cmd | EXPECT: x\") nor a ledger gate, so they were not linted",
		file, strings.Join(parts, ", "))
}

func specID(spec string) string {
	head, _, _ := strings.Cut(spec, "|")
	return gateID(strings.TrimSpace(head), 0)
}

// unwrapSpec은 spec 줄 전체를 감싼 한 쌍의 따옴표나 백틱만 벗긴다. 여는 문자 없이 끝에만
// 있는 백틱(EXPECT를 감싼 백틱)은 spec의 일부이므로 그대로 둔다.
func unwrapSpec(spec, open string) string {
	if open != "" && len(spec) >= 2 && strings.HasSuffix(spec, open) {
		return spec[1 : len(spec)-1]
	}
	return spec
}
