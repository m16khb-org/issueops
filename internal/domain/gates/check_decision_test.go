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
		{"timeout", "ok", 0, true, false, "check timed out: ok"},
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
