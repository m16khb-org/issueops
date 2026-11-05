package guard

import (
	guardcontract "issueops/internal/contract/guard"
	"strings"

	"issueops/internal/domain/guardpattern"
)

func fileFindings(rel, content string, existingSymbols map[string][]string) []guardcontract.Finding {
	findings := []guardcontract.Finding{}
	immutablePrefixBuilder := strings.Contains(content, guardpattern.ImmutablePrefixMarker)
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lineNo := i + 1
		if immutablePrefixBuilder && guardpattern.ContextNonDeterminism().MatchString(line) && !strings.Contains(line, guardpattern.VolatileOKMarker) {
			findings = append(findings, guardcontract.Finding{
				Severity: "warn",
				Rule:     "nondeterministic-context-serialization",
				File:     rel,
				Line:     lineNo,
				Message:  "Immutable-prefix context builder introduces a non-deterministic value; move it to a volatile region or annotate the line with volatile-ok.",
				Evidence: strings.TrimSpace(line),
			})
		}
		if executableTestSourcePath(rel) {
			findings = append(findings, testLineFindings(rel, line, lineNo)...)
		}
		if m := guardpattern.NewSymbol().FindStringSubmatch(line); len(m) == 2 {
			symbol := m[1]
			if reuseFinding, ok := reuseFinding(rel, lineNo, symbol, existingSymbols); ok {
				findings = append(findings, reuseFinding)
			}
		}
	}
	if testPath(rel) && len(content) > 200_000 {
		findings = append(findings, guardcontract.Finding{Severity: "warn", Rule: "large-test-fixture", File: rel, Message: "Large test files or fixtures can hide weak assertions; prefer small named fixtures."})
	}
	return findings
}

// testLineFindings는 실행 가능한 테스트 소스의 한 줄에서 테스트 위생 규칙 위반을 모은다.
func testLineFindings(rel, line string, lineNo int) []guardcontract.Finding {
	var findings []guardcontract.Finding
	if guardpattern.AmbiguousTestName().MatchString(line) {
		findings = append(findings, guardcontract.Finding{Severity: "warn", Rule: "ambiguous-test-name", File: rel, Line: lineNo, Message: "Test name is too generic to communicate the protected contract.", Evidence: strings.TrimSpace(line)})
	}
	if guardpattern.SleepInTest().MatchString(line) {
		findings = append(findings, guardcontract.Finding{Severity: "block", Rule: "sleep-in-test", File: rel, Line: lineNo, Message: "Tests must not depend on wall-clock sleep; use deterministic synchronization or fake clocks.", Evidence: strings.TrimSpace(line)})
	}
	for _, url := range guardpattern.ExternalURL().FindAllString(line, -1) {
		if !allowsFixtureURL(url) {
			findings = append(findings, guardcontract.Finding{Severity: "block", Rule: "real-external-service-in-test", File: rel, Line: lineNo, Message: "Tests must not depend on real external services.", Evidence: url})
		}
	}
	if strings.Contains(strings.ToLower(line), "localhost") {
		findings = append(findings, guardcontract.Finding{Severity: "warn", Rule: "localhost-in-test", File: rel, Line: lineNo, Message: "Local service dependencies in tests need explicit isolation and lifecycle control.", Evidence: strings.TrimSpace(line)})
	}
	if guardpattern.SnapshotAssertion().MatchString(line) {
		findings = append(findings, guardcontract.Finding{Severity: "warn", Rule: "snapshot-test-review", File: rel, Line: lineNo, Message: "Snapshot/golden assertions should be paired with focused contract checks and intentional update notes.", Evidence: strings.TrimSpace(line)})
	}
	return findings
}
