package gates

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestLintStaticFlagsGatesThatCanNeverBeMet(t *testing.T) {
	cases := []struct {
		name     string
		spec     string
		problem  string
		runnable bool // CHECK still reaches the policy judgment, so every problem shows in one pass
	}{
		{"shell chaining is not executed", "G2: x | CHECK: go vet ./... && go test ./... | EXPECT: ok", "one argv command", false},
		{"EXPECT regex that RE2 rejects never matches", "G3: x | CHECK: printf ok | EXPECT: /a(b/", "does not compile", true},
		{"gate without CHECK stays unchecked", "G4: x | EXPECT: ok", "no CHECK", false},
		{"markdown backticks stay in argv", "G5: x | CHECK: `go test ./...` | EXPECT: ok", "markdown backticks", true},
		{"backtick-wrapped EXPECT is matched literally", "G6: x | CHECK: printf ok | EXPECT: `ok`", "wrapped in markdown backticks", true},
		{"anchored regex without slashes is a literal that never matches", `G7: x | CHECK: printf ok | EXPECT: ^ok$`, "write /^ok$/", true},
		{"shell expansion is passed through literally", `G8: x | CHECK: test "$(./hello.sh --version)" = "hello 0.1.0" | EXPECT: ok`, "shell expansion", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, argv := LintStatic(LintSpecCandidate("spec", tc.spec))
			if len(result.Problems) != 1 || !strings.Contains(result.Problems[0], tc.problem) {
				t.Fatalf("problems = %q, want one containing %q", result.Problems, tc.problem)
			}
			if (argv != nil) != tc.runnable {
				t.Fatalf("argv = %q, runnable want %v", argv, tc.runnable)
			}
		})
	}
}

func TestLintStaticPassesRunnableGateAndWarnsOnConsumedBackslashes(t *testing.T) {
	clean, argv := LintStatic(LintSpecCandidate("spec", `G1: x | CHECK: python3 -B -c 'print(r"\d")' | EXPECT: /\\d/`))
	if len(clean.Problems) != 0 || len(clean.Warnings) != 0 {
		t.Fatalf("single-quoted backslash gate: problems %q warnings %q, want none", clean.Problems, clean.Warnings)
	}
	if want := []string{"python3", "-B", "-c", `print(r"\d")`}; !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	consumed, _ := LintStatic(LintSpecCandidate("spec", `G2: x | CHECK: python3 -B -c "import re;print(re.search('\d','1'))" | EXPECT: /Match/`))
	if len(consumed.Problems) != 0 || len(consumed.Warnings) != 1 || !strings.Contains(consumed.Warnings[0], "consumed 1 backslash") {
		t.Fatalf("double-quoted backslash gate: problems %q warnings %q, want one backslash warning", consumed.Problems, consumed.Warnings)
	}
}

func TestPlanLintCandidatesReadsSpecAndLedgerGatesAndReportsTheRest(t *testing.T) {
	plan := strings.Join([]string{
		"## 수용 기준",
		"- G1: tests pass | CHECK: go test ./... | EXPECT: ok",
		"- `G2: docs | CHECK: printf ok | EXPECT: ok`",
		`  --gate "G3: quoted | CHECK: printf \"ok\" | EXPECT: ok" \`,
		"- [ ] G4: ledger form",
		"  CHECK: printf ok",
		"  EXPECT: ok",
		"- G5: `go test ./x` → EXPECT ok (CHECK: decided later)",
	}, "\n")
	candidates, unparsed := PlanLintCandidates("plan.md", plan)
	var ids, sources []string
	for _, candidate := range candidates {
		ids = append(ids, candidate.Gate.ID)
		sources = append(sources, candidate.Source)
	}
	if want := []string{"G1", "G2", "G3", "G4"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %q, want %q", ids, want)
	}
	if want := []string{"plan.md:2", "plan.md:3", "plan.md:4", "plan.md:5"}; !reflect.DeepEqual(sources, want) {
		t.Fatalf("sources = %q, want %q", sources, want)
	}
	if got := candidates[2].Gate.CheckCmd; got != `printf "ok"` {
		t.Fatalf("G3 CHECK = %q, want the --gate quoting removed", got)
	}
	if want := []int{8}; !reflect.DeepEqual(unparsed, want) {
		t.Fatalf("unparsed = %v, want %v", unparsed, want)
	}
}

// Regex syntax in a literal EXPECT is kept as characters; lint says so
// without failing a literal that only happens to contain a dollar sign.
func TestLintStaticWarnsOnRegexLookingLiteralExpect(t *testing.T) {
	for _, expect := range []string{"^hello", "hello 0\\.1", "v1.*", `int f\(const char \*s\);`} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: "+expect))
		if len(result.Problems) != 0 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "looks like a regex") {
			t.Fatalf("%q: problems=%q warnings=%q", expect, result.Problems, result.Warnings)
		}
	}
	for _, expect := range []string{"cost 5\\$", "ok", "/^ok$/", "/^ok/m"} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: "+expect))
		if len(result.Problems) != 0 || len(result.Warnings) != 0 {
			t.Fatalf("%q: problems=%q warnings=%q", expect, result.Problems, result.Warnings)
		}
	}
}

// QA: EXPECT "exit 0" on a silent assertion failed the gate; EXPECT reads
// output lines and exit code 0 is always required.
func TestLintStaticWarnsWhenExpectDescribesAnExitStatus(t *testing.T) {
	for _, expect := range []string{"exit 0", "Exit code 0", "exit status: 0", "rc=0", "return code 0"} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: "+expect))
		if len(result.Problems) != 0 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "not its exit status") {
			t.Fatalf("%q: problems=%q warnings=%q", expect, result.Problems, result.Warnings)
		}
	}
	for _, expect := range []string{"0", "exit 1", "exited cleanly", "/exit 0/"} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: "+expect))
		for _, warning := range result.Warnings {
			if strings.Contains(warning, "not its exit status") {
				t.Fatalf("%q must not warn: %q", expect, warning)
			}
		}
	}
}

// `sh script.sh` (the form project docs often give) is denied; the message
// names the form that runs. Denials without a remedy carry no hint.
func TestDeniedMessageNamesTheShellInterpreterRemedy(t *testing.T) {
	if got := DeniedMessage([]string{"command_not_in_read_only_allowlist", "shell_interpreter_not_allowed"}); !strings.Contains(got, "run an executable script directly") {
		t.Fatalf("shell denial: %q", got)
	}
	if got := DeniedMessage([]string{"write_not_allowed"}); got != "check denied by policy: write_not_allowed" {
		t.Fatalf("other denial: %q", got)
	}
	// A Codex run checked a worktree ledger with the source checkout as root.
	if got := DeniedMessage([]string{"cwd_outside_workspace", "path_outside_workspace"}); !strings.Contains(got, "canonical worktree as both") {
		t.Fatalf("workspace denial: %q", got)
	}
}

// An agent's edit left two G2 gates; lint passed both, and check/abandon,
// which address gates by ID, could reach only one.
func TestLedgerLintCandidatesFlagsRepeatedGateID(t *testing.T) {
	text := "- [ ] G1: a\n  CHECK: echo a\n- [ ] G2: b\n  CHECK: echo b\n- [ ] G2: c\n  CHECK: echo c\n"
	got := LedgerLintCandidates("gates.md", text)
	if len(got) != 3 || len(got[1].Problems) != 0 || len(got[2].Problems) != 1 || !strings.Contains(got[2].Problems[0], "gate ID G2 repeats line 3") {
		t.Fatalf("candidates = %+v", got)
	}
	if result, argv := LintStatic(got[2]); argv != nil || len(result.Problems) != 1 {
		t.Fatalf("lint result = %+v argv=%v", result, argv)
	}
}

// `/\Ahello\n\z/` linted clean and then never matched: check trims trailing
// newlines before matching.
func TestLintStaticFlagsNewlineBeforeEndAnchor(t *testing.T) {
	gate := Gate{ID: "G1", CheckCmd: "./hello.sh", Expect: `/\Ahello\n\z/`}
	if DecideCheck(gate.Expect, "hello\n", "", 0, false).Passed {
		t.Fatal("premise: the pattern matches trimmed output")
	}
	result, _ := LintStatic(LintCandidate{Source: "g.md", Gate: gate})
	if len(result.Problems) != 1 || !strings.Contains(result.Problems[0], `drop the \n before \z`) {
		t.Fatalf("problems = %q", result.Problems)
	}
}

func TestDeniedMessageNamesTheNetworkRemedy(t *testing.T) {
	if got := DeniedMessage([]string{"network_not_allowed"}); !strings.Contains(got, "node --test") || !strings.Contains(got, "--network") {
		t.Fatalf("message %q", got)
	}
}

func TestLintWarnsAboutPythonWithoutDashB(t *testing.T) {
	for check, warns := range map[string]bool{
		"python3 -c 'print(1)'":                    true,
		"python3 -m unittest discover -s tests":    true,
		"python3 -B -m unittest discover -s tests": false,
		"python3 -Bc 'print(1)'":                   false,
		"/usr/bin/python3.12 tests/run.py":         true,
		"node --test":                              false,
	} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: "+check+" | EXPECT: 1"))
		got := slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.Contains(w, "python3 -B") })
		if got != warns {
			t.Fatalf("%q: warned=%v (%v)", check, got, result.Warnings)
		}
	}
}

// Rust QA: /^test result: ok/ anchors the start of the whole output, and
// cargo's summary line is never first.
func TestLintStaticWarnsOnLineAnchorWithoutMultilineFlag(t *testing.T) {
	result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: /^test result: ok/"))
	if len(result.Problems) != 0 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "no m flag") {
		t.Fatalf("problems=%q warnings=%q", result.Problems, result.Warnings)
	}
}

// Rust QA: a name filter that matches nothing runs zero tests and still
// succeeds; lint asks for a count or the test's own line in EXPECT.
func TestLintStaticWarnsOnNameFilteredCheckWithoutCountOrName(t *testing.T) {
	for _, spec := range []string{
		"cargo test char_count | EXPECT: /^test result: ok/m",
		"go test -run TestX ./... | EXPECT: ok",
		"python3 -B -m pytest -k dec | EXPECT: /passed/",
		"npx jest -t adds | EXPECT: /Tests:/",
	} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: "+spec))
		if !slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.Contains(w, "selects tests by name") }) {
			t.Fatalf("%q: warnings=%q", spec, result.Warnings)
		}
	}
	for _, spec := range []string{
		"cargo test counts_chars | EXPECT: /^test result: ok\\. [1-9]/m",
		"go test -v -run TestDec ./... | EXPECT: /--- PASS: TestDec/",
		"cargo test --offline | EXPECT: /^test result: ok/m",
	} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: "+spec))
		if slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.Contains(w, "selects tests by name") }) {
			t.Fatalf("%q: warnings=%q", spec, result.Warnings)
		}
	}
}

// QA (README typo fix): an unquoted multi-word grep pattern lint said clean.
func TestLintWarnsAboutAnUnquotedMultiWordSearchPattern(t *testing.T) {
	for check, warns := range map[string]bool{
		"grep -c This library adds numbers README.md":   true,
		"rg -c foo bar src/x.go":                        true,
		"grep -c 'This library adds numbers' README.md": false,
		"grep -r foo src lib":                           false,
		"grep -m 1 foo README.md":                       false,
		"grep -e foo -e bar README.md docs/a.md":        false,
		"grep foo README.md docs/a.md":                  false,
	} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: "+check+" | EXPECT: 1"))
		got := slices.ContainsFunc(result.Warnings, func(w string) bool { return strings.Contains(w, "multi-word pattern") })
		if got != warns {
			t.Fatalf("%q: warned=%v (%v)", check, got, result.Warnings)
		}
	}
}
