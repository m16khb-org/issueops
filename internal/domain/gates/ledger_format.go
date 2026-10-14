package gates

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	// inlineSpecRe는 체크박스 줄에 붙은 gates init spec 형식의 `| CHECK:`다.
	inlineSpecRe = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`\|\s*CHECK:`) })
	// listAttrLineRe는 목록 항목으로 적은 속성(`  - EVIDENCE: ...`, 들여쓰지 않은 `- EVIDENCE: ...`)이다.
	// 파서는 목록 기호 없는 들여쓴 속성 줄만 읽으므로 이 줄은 게이트에 붙지 않는다.
	listAttrLineRe = sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(`^[ \t]*[-*][ \t]+(CHECK|EXPECT|EVIDENCE):`)
	})
)

// LedgerFormatWarnings는 원장에 적었지만 파서가 읽지 못하는 게이트 속성을 짚는다. 이런 게이트는
// 체크박스가 [x]여도 CHECK가 없거나 EVIDENCE가 pending으로 읽혀 met이 되지 않는다. 경고는
// 왜 그런지(어디에 적어야 읽히는지)를 말해, 결과만 보고 원인을 추측하지 않게 한다.
func LedgerFormatWarnings(file string, ledger Ledger) []string {
	var warnings []string
	for i, gate := range ledger.Gates {
		if strings.TrimSpace(gate.CheckCmd) == "" && inlineSpecRe().MatchString(gate.Title) {
			warnings = append(warnings, fmt.Sprintf(
				"%s %s: CHECK/EXPECT are on the checkbox line (the gates init --gate spec form); a ledger reads them only from indented `CHECK:`/`EXPECT:` lines under the gate, so this gate has no CHECK",
				file, gate.ID))
		}
		end := len(ledger.Lines)
		if i+1 < len(ledger.Gates) {
			end = ledger.Gates[i+1].CheckboxLine
		}
		reported := map[string]bool{}
		for _, line := range ledger.Lines[gate.CheckboxLine+1 : end] {
			if strings.HasPrefix(strings.TrimRight(line, "\r"), "#") {
				break
			}
			m := listAttrLineRe().FindStringSubmatch(line)
			if m == nil || reported[m[1]] {
				continue
			}
			reported[m[1]] = true
			warnings = append(warnings, fmt.Sprintf(
				"%s %s: %s is written as a list item (`- %s:`); a ledger reads only indented `%s:` lines without a list marker, so it is ignored",
				file, gate.ID, m[1], m[1], m[1]))
		}
	}
	return warnings
}
