package gates

import (
	"reflect"
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
		{"pipe inside an EXPECT regex splits the spec", "G1: x | CHECK: printf ok | EXPECT: /a|b/", `unknown segment "b/"`, false},
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
	clean, argv := LintStatic(LintSpecCandidate("spec", `G1: x | CHECK: python3 -c 'print(r"\d")' | EXPECT: /\\d/`))
	if len(clean.Problems) != 0 || len(clean.Warnings) != 0 {
		t.Fatalf("single-quoted backslash gate: problems %q warnings %q, want none", clean.Problems, clean.Warnings)
	}
	if want := []string{"python3", "-c", `print(r"\d")`}; !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %q, want %q", argv, want)
	}
	consumed, _ := LintStatic(LintSpecCandidate("spec", `G2: x | CHECK: python3 -c "import re;print(re.search('\d','1'))" | EXPECT: /Match/`))
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
	for _, expect := range []string{"^hello", "hello 0\\.1", "v1.*"} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: "+expect))
		if len(result.Problems) != 0 || len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "looks like a regex") {
			t.Fatalf("%q: problems=%q warnings=%q", expect, result.Problems, result.Warnings)
		}
	}
	for _, expect := range []string{"cost 5\\$", "ok", "/^ok$/"} {
		result, _ := LintStatic(LintSpecCandidate("spec", "G1: x | CHECK: printf ok | EXPECT: "+expect))
		if len(result.Problems) != 0 || len(result.Warnings) != 0 {
			t.Fatalf("%q: problems=%q warnings=%q", expect, result.Problems, result.Warnings)
		}
	}
}

// `sh script.sh` (the form project docs often give) is denied; the message
// names the form that runs. Other denials carry no such hint.
func TestDeniedMessageNamesTheShellInterpreterRemedy(t *testing.T) {
	if got := DeniedMessage([]string{"command_not_in_read_only_allowlist", "shell_interpreter_not_allowed"}); !strings.Contains(got, "run an executable script directly") {
		t.Fatalf("shell denial: %q", got)
	}
	if got := DeniedMessage([]string{"network_not_allowed"}); got != "check denied by policy: network_not_allowed" {
		t.Fatalf("other denial: %q", got)
	}
}
