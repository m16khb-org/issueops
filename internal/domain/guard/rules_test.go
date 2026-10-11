package guard

import (
	"slices"
	"strings"
	"testing"

	guardcontract "issueops/internal/contract/guard"
)

func ruleLines(findings []guardcontract.GuardFinding, rule string) []int {
	var lines []int
	for _, finding := range findings {
		if finding.Rule == rule {
			lines = append(lines, finding.Line)
		}
	}
	return lines
}

func TestModeFallsBackByRequestShape(t *testing.T) {
	cases := map[string]struct {
		request guardcontract.GuardCheckRequest
		want    string
	}{
		"staged flag":        {guardcontract.GuardCheckRequest{Staged: true}, "staged"},
		"explicit files":     {guardcontract.GuardCheckRequest{Files: []string{"a.go"}}, "files"},
		"empty request":      {guardcontract.GuardCheckRequest{}, "staged"},
		"all beats staged":   {guardcontract.GuardCheckRequest{All: true, Staged: true}, "all"},
		"staged beats files": {guardcontract.GuardCheckRequest{Staged: true, Files: []string{"a.go"}}, "staged"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Mode(tc.request); got != tc.want {
				t.Fatalf("Mode = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAllowsFixtureURLAcceptsOnlyReservedHosts(t *testing.T) {
	cases := map[string]bool{
		"https://example.com/path":           true,
		"https://EXAMPLE.ORG":                true,
		"http://example.net:8080/x":          true,
		"https://svc.example.invalid/x":      false, // subdomains are not reserved
		"https://example.invalid/x":          true,
		"http://127.0.0.1:9000/health":       true,
		"http://localhost:3000":              true,
		"https://github.com/example/repo":    true,
		"https://github.com/Example/Repo":    true,
		"https://github.com/real-org/repo":   false,
		"https://github.com/examples/repo":   false, // prefix is /example/, not /example
		"https://api.stripe.com/v1/charges":  false,
		"https://www.example.com/":           false,
		"https://evil.test/?u=example.com":   false,
		"http://[::1":                        false, // unparseable
		"https://user@evil.test/example.com": false,
	}
	for raw, want := range cases {
		if got := AllowsFixtureURL(raw); got != want {
			t.Errorf("AllowsFixtureURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestSeverityRankOrdersBlockBeforeWarnBeforeReview(t *testing.T) {
	order := []string{"block", "warn", "review", "info"}
	for i := 1; i < len(order); i++ {
		if SeverityRank(order[i-1]) >= SeverityRank(order[i]) {
			t.Fatalf("rank(%s)=%d must be below rank(%s)=%d", order[i-1], SeverityRank(order[i-1]), order[i], SeverityRank(order[i]))
		}
	}
	if SeverityRank("mystery") != SeverityRank("info") {
		t.Fatal("unknown severities must rank with info (least severe)")
	}
}

func TestDedupeFindingsKeepsFirstOfIdenticalKeysAndDistinctLines(t *testing.T) {
	first := guardcontract.GuardFinding{Severity: "warn", Rule: "r", File: "f.go", Line: 3, Evidence: "e", Message: "first"}
	dup := first
	dup.Message = "second"
	otherLine := first
	otherLine.Line = 4
	otherSeverity := first
	otherSeverity.Severity = "block"
	got := DedupeFindings([]guardcontract.GuardFinding{first, dup, otherLine, otherSeverity})
	if len(got) != 3 || got[0].Message != "first" || got[1].Line != 4 || got[2].Severity != "block" {
		t.Fatalf("DedupeFindings = %+v", got)
	}
	if empty := DedupeFindings(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("DedupeFindings(nil) = %#v, want empty non-nil slice", empty)
	}
}

func TestFindingLessOrdersBySeverityFileLineThenRule(t *testing.T) {
	f := func(sev, file string, line int, rule string) guardcontract.GuardFinding {
		return guardcontract.GuardFinding{Severity: sev, File: file, Line: line, Rule: rule}
	}
	ordered := []guardcontract.GuardFinding{
		f("block", "z.go", 9, "z"),
		f("warn", "a.go", 1, "a"),
		f("warn", "b.go", 1, "a"),
		f("warn", "b.go", 2, "a"),
		f("warn", "b.go", 2, "b"),
		f("review", "a.go", 1, "a"),
	}
	for i := range ordered {
		for j := range ordered {
			if got, want := findingLess(ordered[i], ordered[j]), i < j; got != want {
				t.Errorf("findingLess(#%d, #%d) = %v, want %v", i, j, got, want)
			}
		}
	}
}

func TestSummarizeFindingsCountsEachSeverityAndTreatsUnknownAsInfo(t *testing.T) {
	got := summarizeFindings([]guardcontract.GuardFinding{
		{Severity: "block"}, {Severity: "warn"}, {Severity: "warn"}, {Severity: "review"}, {Severity: "info"}, {Severity: "weird"},
	})
	if got.Block != 1 || got.Warn != 2 || got.Review != 1 || got.Info != 2 {
		t.Fatalf("summary = %+v", got)
	}
}

func TestAnalyzeSecretPathShortCircuitsContentAndChangeClassification(t *testing.T) {
	got := Analyze([]FileObservation{
		{Path: "config/secret.go", Content: "func TestWorks() { time.Sleep(1) }", Read: true},
	}, nil)
	if got.OK || len(got.Findings) != 1 || got.Findings[0].Rule != "secret-like-path" || got.Findings[0].File != "config/secret.go" {
		t.Fatalf("secret path must yield only its own block finding: %+v", got)
	}
	// An executable-test path would otherwise report sleep/ambiguous-name findings.
	got = Analyze([]FileObservation{
		{Path: "config/secret_test.go", Content: "func TestWorks() { time.Sleep(1) }", Read: true},
	}, nil)
	if len(got.Findings) != 1 || got.Findings[0].Rule != "secret-like-path" {
		t.Fatalf("secret test path content must not be analyzed: %+v", got.Findings)
	}
}

func TestAnalyzeIgnoresContentOfUnreadFiles(t *testing.T) {
	got := Analyze([]FileObservation{{Path: "pkg/a_test.go", Content: "time.Sleep(time.Second)", Read: false}}, nil)
	if !got.OK || len(got.Findings) != 0 {
		t.Fatalf("unread content must not be analyzed: %+v", got)
	}
}

func TestAnalyzeProductionChangeWarningDependsOnChangedTest(t *testing.T) {
	prod := FileObservation{Path: "pkg/handler.go"}
	test := FileObservation{Path: "pkg/handler_test.go"}
	if got := Analyze([]FileObservation{prod, test}, nil); hasRule(got.Findings, "prod-change-without-test") {
		t.Fatalf("a changed test file must suppress the warning: %+v", got.Findings)
	}
	if got := Analyze([]FileObservation{test}, nil); hasRule(got.Findings, "prod-change-without-test") {
		t.Fatalf("test-only change must not warn: %+v", got.Findings)
	}
	if got := Analyze([]FileObservation{{Path: "docs/guide.md"}, {Path: ".issueops/X.md"}}, nil); len(got.Findings) != 0 {
		t.Fatalf("docs are not production source: %+v", got.Findings)
	}
}

func TestAnalyzeContractSurfaceWarnsUnlessGoldenOrTestdataChanges(t *testing.T) {
	surface := FileObservation{Path: "internal/adapter/cli/run.go"}
	got := Analyze([]FileObservation{surface}, nil)
	if !hasRule(got.Findings, "contract-surface-without-golden") || !hasRule(got.Findings, "prod-change-without-test") {
		t.Fatalf("surface change alone must warn on both rules: %+v", got.Findings)
	}
	for name, companion := range map[string]string{
		"testdata directory": "internal/adapter/cli/testdata/run.json",
		"golden file name":   "internal/adapter/cli/GOLDEN_run.txt",
	} {
		t.Run(name, func(t *testing.T) {
			got := Analyze([]FileObservation{surface, {Path: companion}}, nil)
			if hasRule(got.Findings, "contract-surface-without-golden") {
				t.Fatalf("%s must satisfy the golden requirement: %+v", companion, got.Findings)
			}
		})
	}
	if got := Analyze([]FileObservation{{Path: "cmd/issueops/main.go"}, {Path: "cmd/issueops/main_test.go"}}, nil); !hasRule(got.Findings, "contract-surface-without-golden") {
		t.Fatalf("a changed test is not a golden update: %+v", got.Findings)
	}
}

func TestAnalyzeOrdersFindingsAndOnlyBlocksOnBlockSeverity(t *testing.T) {
	sleepy := "package p\nfunc TestParses(t *testing.T) {\n\ttime.Sleep(1)\n}\n"
	got := Analyze([]FileObservation{
		{Path: "pkg/b_test.go", Content: sleepy, Read: true},
		{Path: "pkg/a_test.go", Content: sleepy, Read: true},
		{Path: "pkg/c_test.go", Content: "x := \"http://localhost:1\"\n", Read: true},
	}, nil)
	if got.OK {
		t.Fatalf("sleep-in-test must block: %+v", got)
	}
	var order []string
	for _, finding := range got.Findings {
		order = append(order, finding.Severity+":"+finding.File)
	}
	want := []string{"block:pkg/a_test.go", "block:pkg/b_test.go", "warn:pkg/c_test.go"}
	if !slices.Equal(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if got.Summary.Block != 2 || got.Summary.Warn != 1 {
		t.Fatalf("summary = %+v", got.Summary)
	}

	warnOnly := Analyze([]FileObservation{{Path: "pkg/c_test.go", Content: "localhost\n", Read: true}}, nil)
	if !warnOnly.OK || warnOnly.Summary.Block != 0 || warnOnly.Summary.Warn != 1 {
		t.Fatalf("warnings alone must keep OK: %+v", warnOnly)
	}
}

func TestAnalyzeReportsReuseCandidateAsReviewWithoutBlocking(t *testing.T) {
	existing := map[string][]string{NormalizeSymbol("parse_config"): {"internal/old/config.go:ParseConfig"}}
	got := Analyze([]FileObservation{
		{Path: "pkg/x_test.go", Content: "func ParseConfigs() {}\nfunc Unrelated() {}\n", Read: true},
	}, existing)
	if !got.OK || got.Summary.Review != 1 {
		t.Fatalf("analysis = %+v", got)
	}
	finding := got.Findings[0]
	if finding.Rule != "reuse-before-new" || finding.Line != 1 || finding.Evidence != "ParseConfigs" {
		t.Fatalf("finding = %+v", finding)
	}
}

func TestFileFindingsFlagsNonDeterminismOnlyInImmutablePrefixBuilders(t *testing.T) {
	builder := "// harness:immutable-prefix\nstamp := time.Now()\nok := time.Now() // volatile-ok\nid := uuid.New()\n"
	got := FileFindings("pkg/ctx.go", builder, nil)
	if lines := ruleLines(got, "nondeterministic-context-serialization"); !slices.Equal(lines, []int{2, 4}) {
		t.Fatalf("flagged lines = %v, want [2 4] (volatile-ok line exempt)", lines)
	}
	plain := FileFindings("pkg/ctx.go", "stamp := time.Now()\n", nil)
	if len(plain) != 0 {
		t.Fatalf("files without the marker are not checked: %+v", plain)
	}
}

func TestFileFindingsLargeFixtureThresholdIsExclusive(t *testing.T) {
	if got := FileFindings("testdata/big.json", strings.Repeat("a", 200_000), nil); hasRule(got, "large-test-fixture") {
		t.Fatalf("exactly 200000 bytes must not warn: %+v", got)
	}
	if got := FileFindings("testdata/big.json", strings.Repeat("a", 200_001), nil); !hasRule(got, "large-test-fixture") {
		t.Fatal("200001 bytes in a test path must warn")
	}
	if got := FileFindings("pkg/big.go", strings.Repeat("a", 200_001), nil); hasRule(got, "large-test-fixture") {
		t.Fatal("non-test paths must not get the fixture warning")
	}
}

func TestFileFindingsAppliesTestHygieneRulesOnlyToExecutableTestSources(t *testing.T) {
	content := strings.Join([]string{
		"package p",
		"func TestBasic(t *testing.T) {",
		"\thttp.Get(\"https://api.stripe.com/v1\")",
		"\thttp.Get(\"https://example.com/ok\")",
		"\tdial(\"localhost:5432\")",
		"\tcupaloy.toMatchSnapshot(x)",
		"\ttime.Sleep(time.Second)",
	}, "\n")
	got := FileFindings("pkg/svc_test.go", content, nil)
	for rule, wantLine := range map[string]int{
		"ambiguous-test-name":           2,
		"real-external-service-in-test": 3,
		"localhost-in-test":             5,
		"snapshot-test-review":          6,
		"sleep-in-test":                 7,
	} {
		if lines := ruleLines(got, rule); !slices.Equal(lines, []int{wantLine}) {
			t.Errorf("%s lines = %v, want [%d]", rule, lines, wantLine)
		}
	}
	for _, finding := range got {
		if finding.Rule == "real-external-service-in-test" && finding.Evidence != "https://api.stripe.com/v1" {
			t.Errorf("only the real URL may be reported, got evidence %q", finding.Evidence)
		}
	}

	for _, path := range []string{"pkg/testdata/svc_test.go", "pkg/fixtures/svc_test.go", "pkg/svc.go"} {
		if got := FileFindings(path, content, nil); len(got) != 0 {
			t.Errorf("%s is not an executable test source, got %+v", path, got)
		}
	}
}

func TestExecutableTestSourcePathRequiresCodeExtensionAndTestName(t *testing.T) {
	cases := map[string]bool{
		"pkg/foo_test.go":          true,
		"web/app.spec.ts":          true,
		"PKG/Foo_TEST.GO":          true,
		"pkg/foo.go":               false,
		"docs/test-plan.md":        false,
		"pkg/testdata/foo_test.go": false,
		"pkg/foo_test.golden.go":   false,
		"pkg/fixture/foo_test.go":  false,
		"pkg/fixtures/foo_test.go": false,
	}
	for path, want := range cases {
		if got := ExecutableTestSourcePath(path); got != want {
			t.Errorf("ExecutableTestSourcePath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestRelevantPathCoversExtensionlessAndCaseVariants(t *testing.T) {
	cases := map[string]bool{
		"build/Dockerfile":  true,
		"testdata/snapshot": true,
		"Makefile":          false,
		"src/Main.GO":       true,
		"deploy/app.yaml":   true,
		"logo.svg":          false,
		"keys/server.pem":   true,
	}
	for path, want := range cases {
		if got := RelevantPath(path); got != want {
			t.Errorf("RelevantPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestSourcePathExcludesTestsDocsAndNonCode(t *testing.T) {
	cases := map[string]bool{
		"pkg/main.go":      true,
		"pkg/Main.GO":      true,
		"db/schema.sql":    true,
		".issueops/run.sh": false,
		"docs/tool.py":     false,
		"pkg/main_test.go": false,
		"README.md":        false,
	}
	for path, want := range cases {
		if got := SourcePath(path); got != want {
			t.Errorf("SourcePath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestNormalizeSymbolIgnoresCaseStyleOrderAndPlurals(t *testing.T) {
	canonical := NormalizeSymbol("ParseConfig")
	if canonical == "" {
		t.Fatal("ParseConfig must normalize to a non-empty key")
	}
	for _, variant := range []string{"parse_config", "parse-config", "config_parse", "ParseConfigs", "  parseConfig  "} {
		if got := NormalizeSymbol(variant); got != canonical {
			t.Errorf("NormalizeSymbol(%q) = %q, want %q", variant, got, canonical)
		}
	}
	if NormalizeSymbol("ParseConfig") == NormalizeSymbol("ParseLedger") {
		t.Fatal("different symbols must not collide")
	}
}

func TestNormalizeSymbolDropsShortTokens(t *testing.T) {
	for _, symbol := range []string{"", "   ", "a_b", "Go", "x_y_z"} {
		if got := NormalizeSymbol(symbol); got != "" {
			t.Errorf("NormalizeSymbol(%q) = %q, want empty", symbol, got)
		}
	}
	if NormalizeSymbol("my_parse") != NormalizeSymbol("parse") {
		t.Fatal("two-letter tokens must be dropped from the key")
	}
}

func TestReuseFindingRequiresKnownCandidates(t *testing.T) {
	existing := map[string][]string{
		NormalizeSymbol("load_settings"): {"a.go:LoadSettings", "b.go:load_settings"},
		NormalizeSymbol("empty_slot"):    {},
		"":                               {"z.go:Ab"},
	}
	if _, ok := ReuseFinding("x.go", 1, "BrandNewThing", existing); ok {
		t.Fatal("unknown symbol must not produce a finding")
	}
	if _, ok := ReuseFinding("x.go", 1, "EmptySlot", existing); ok {
		t.Fatal("a key with no candidates must not produce a finding")
	}
	if _, ok := ReuseFinding("x.go", 1, "ab", existing); ok {
		t.Fatal("a symbol that normalizes to empty must not produce a finding")
	}
	finding, ok := ReuseFinding("x.go", 7, "LoadSettings", existing)
	if !ok || finding.Severity != "review" || finding.File != "x.go" || finding.Line != 7 || finding.Evidence != "LoadSettings" {
		t.Fatalf("finding = %+v ok=%v", finding, ok)
	}
	if len(finding.Suggestions) != 2 || !strings.Contains(finding.Suggestions[0], "a.go:LoadSettings, b.go:load_settings") || !strings.Contains(finding.Suggestions[1], `"LoadSettings"`) {
		t.Fatalf("suggestions = %q", finding.Suggestions)
	}
}
