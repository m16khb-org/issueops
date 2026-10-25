package gates

import (
	"strings"
	"testing"
)

func TestShouldRunRequiresUncheckedOrPendingEvidence(t *testing.T) {
	if !ShouldRun(Gate{}) || !ShouldRun(Gate{Checked: true, Evidence: "pending"}) || ShouldRun(Gate{Checked: true, Evidence: "measured"}) {
		t.Fatal("gate rerun eligibility changed")
	}
}

func TestDecideCheckRequiresExitZeroAndMatchingExpect(t *testing.T) {
	cases := []struct {
		name, expect string
		exit         int
		timedOut     bool
		wantPass     bool
		wantError    string
	}{
		{"matched", "ok", 0, false, true, ""},
		{"nonzero with match", "ok", 1, false, false, "exit code 1: ok"},
		{"missing match", "other", 0, false, false, "expect not matched: ok"},
		{"regex-looking literal names the regex form", "^o.*", 0, false, false, "expect not matched: ok (EXPECT is matched as a literal line; write /^o.*/ for a regex)"},
		{"timeout names the limit flag", "ok", 0, true, false, "check timed out: ok (if the command is legitimately slow, rerun gates check with a larger --timeout-seconds (MCP timeout_seconds); default 120)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideCheck(tc.expect, "ok\n", "", tc.exit, tc.timedOut)
			if got.Passed != tc.wantPass || got.Error != tc.wantError || got.Evidence != "ok" {
				t.Fatalf("decision = %+v", got)
			}
		})
	}
}

func TestDecideCheckJoinsOutputAndKeepsEvidenceTail(t *testing.T) {
	got := DecideCheck("", "first\n", "second\n", 0, false)
	if !got.Passed || got.Evidence != "first | second" {
		t.Fatalf("decision = %+v", got)
	}
}

func TestCheckArgvRejectsUnquotedShellControlOperators(t *testing.T) {
	if _, err := CheckArgv("printf ok ; touch marker"); err == "" {
		t.Fatal("shell separator was accepted")
	}
	argv, err := CheckArgv("printf 'ok; still one argument'")
	if err != "" || len(argv) != 2 || argv[1] != "ok; still one argument" {
		t.Fatalf("quoted shell character was rejected: argv=%v err=%q", argv, err)
	}
}

// An agent's `HELLO_GREETING=hi ./hello.sh` linted clean; without a shell the
// assignment is the program name, so the gate could never pass.
func TestCheckArgvRejectsLeadingEnvironmentAssignment(t *testing.T) {
	if _, err := CheckArgv("HELLO_GREETING=hi ./hello.sh"); !strings.Contains(err, "write env HELLO_GREETING=hi ./hello.sh") {
		t.Fatalf("err = %q", err)
	}
	if argv, err := CheckArgv("env HELLO_GREETING=hi ./hello.sh"); err != "" || argv[0] != "env" {
		t.Fatalf("env form refused: %v %q", argv, err)
	}
	if _, err := CheckArgv("go test -run=TestX ./..."); err != "" {
		t.Fatalf("a flag with = in a later argument is not an assignment: %q", err)
	}
}

// A prefix followed by punctuation does not match; the refusal names the line
// and the two forms that do.
func TestDecideCheckNamesALinePrefixThatIsNotFollowedByABlank(t *testing.T) {
	got := DecideCheck("PASS", "PASS: --lower rejects extra arguments\n", "", 0, false)
	want := `expect not matched: PASS: --lower rejects extra arguments (the line "PASS: --lower rejects extra arguments" starts with EXPECT but a prefix matches only when a space or tab follows; write the whole line or /^PASS/m)`
	if got.Passed || got.Error != want {
		t.Fatalf("decision = %+v", got)
	}
	// The suggested regex must match when the line is not the first one.
	output := "running 1 test\ntest result: ok. 1 passed; 0 failed\n"
	if !ExpectMatches(`/^test result: ok/m`, output) || DecideCheck("test result: ok", output, "", 0, false).Passed {
		t.Fatal("the suggested /^…/m form must match a later line")
	}
}

// A literal found mid-line (grep -n prefixes the line number) does not match;
// the refusal names the line and the regex form that does.
func TestDecideCheckNamesALineThatContainsExpectMidLine(t *testing.T) {
	got := DecideCheck("mod", "5:Operations: add, div, mod\n", "", 0, false)
	want := `expect not matched: 5:Operations: add, div, mod (the line "5:Operations: add, div, mod" contains EXPECT mid-line, but a literal EXPECT matches a whole line or a line prefix followed by a space; write /mod/ to match anywhere)`
	if got.Passed || got.Error != want {
		t.Fatalf("decision = %+v", got)
	}
}
