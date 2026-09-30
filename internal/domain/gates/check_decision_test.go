package gates

import "testing"

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
