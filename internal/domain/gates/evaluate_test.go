package gates

import (
	"strings"
	"testing"
)

func TestExpectMatchesLineAnchored(t *testing.T) {
	if !expectMatches("3/3 tiers ok", "checking...\n3/3 tiers ok\n") {
		t.Fatal("whole-line match failed")
	}
	if !expectMatches("ok", "ok  \tissueops/internal/adapter/gates\t0.3s") {
		t.Fatal("EXPECT followed by whitespace must match (go test lines)")
	}
	if !expectMatches("docs-ok", "Skill is valid!\n  docs-ok  \n") {
		t.Fatal("trimmed line equal to EXPECT must match")
	}
	if expectMatches("docs-ok", "echo: error: SKILL.md not found\ndocs-ok: error: SKILL.md not found") {
		t.Fatal("EXPECT followed by ':' inside an error line must not match (#484)")
	}
	if expectMatches("ok", "suite: 8/8 passed ok\nbroken") {
		t.Fatal("EXPECT in the middle of a line must not match; use /regex/ for that")
	}
	if expectMatches("done", "starting...") {
		t.Fatal("absent text must not match")
	}
}

func TestExpectMatchesRegexForm(t *testing.T) {
	if !expectMatches("/8\\/8 passed/", "suite: 8/8 passed") {
		t.Fatal("escaped regex match failed")
	}
	if !expectMatches("/all (ok|good)/i", "ALL GOOD") {
		t.Fatal("case-insensitive flag failed")
	}
	if expectMatches("/ok/", "no match here") {
		t.Fatal("regex matched absent text")
	}
	if expectMatches("/([unclosed/", "anything") {
		t.Fatal("invalid regex must evaluate false, not panic")
	}
	if !expectMatches("/^first$/m", "ignored\nfirst\nlast") {
		t.Fatal("multiline flag failed")
	}
}

func TestEvidenceTailKeepsLastTwoNonEmptyLines(t *testing.T) {
	got := evidenceTail("line1\nline2\n\nline3\n  line4  \n", 200)
	if got != "line3 | line4" {
		t.Fatalf("tail = %q, want %q", got, "line3 | line4")
	}
	if got := evidenceTail("", 200); got != "(no output)" {
		t.Fatalf("empty tail = %q", got)
	}
	long := evidenceTail(strings.Repeat("x", 300), 200)
	if len(long) != 200 {
		t.Fatalf("tail cap = %d, want 200", len(long))
	}
}

func TestEvidencePending(t *testing.T) {
	for _, pending := range []string{"", "pending", "PENDING", "  pending  "} {
		if !evidencePending(pending) {
			t.Fatalf("%q must be pending", pending)
		}
	}
	if evidencePending("measured 3/3") {
		t.Fatal("measured evidence must not be pending")
	}
}

func TestStateTransitions(t *testing.T) {
	cases := []struct {
		name string
		gate Gate
		want string
	}{
		{"abandoned wins", Gate{Checked: false, Abandoned: true}, stateAbandoned},
		{"unchecked", Gate{}, stateUnchecked},
		{"checked without evidence", Gate{Checked: true, Evidence: "pending"}, stateEvidencePending},
		{"checked with empty evidence", Gate{Checked: true}, stateEvidencePending},
		{"met", Gate{Checked: true, Evidence: "measured"}, stateMet},
	}
	for _, tc := range cases {
		if got := state(tc.gate); got != tc.want {
			t.Fatalf("%s: state = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSummarizeCountsAndComplete(t *testing.T) {
	summary := summarize([]Gate{
		{Checked: true, Evidence: "measured"},
		{},
		{Checked: true, Evidence: "pending"},
		{Abandoned: true, AbandonReason: "reason"},
	})
	if summary.Total != 4 || summary.Met != 1 || summary.Unmet != 2 || summary.Abandoned != 1 {
		t.Fatalf("summary counts wrong: %+v", summary)
	}
	if summary.Complete {
		t.Fatal("unmet>0 must not be complete")
	}
	allResolved := summarize([]Gate{
		{Checked: true, Evidence: "measured"},
		{Abandoned: true},
	})
	if !allResolved.Complete || allResolved.Met != 1 || allResolved.Abandoned != 1 {
		t.Fatalf("met+abandoned must be complete: %+v", allResolved)
	}
}

// A passing gate's evidence shows the line its EXPECT matched, not only the
// output tail (QA: "PASS | ok pkg" for EXPECT /--- PASS: TestReverse/).
func TestDecideCheckEvidenceShowsTheMatchedLine(t *testing.T) {
	out := "=== RUN   TestReverse\n--- PASS: TestReverse (0.00s)\nPASS\nok  \texample.com/hello/strutil\t0.079s\n"
	got := decideCheck("/--- PASS: TestReverse/", out, "", 0, false)
	if !got.Passed || got.Evidence != "--- PASS: TestReverse (0.00s) | PASS | ok  \texample.com/hello/strutil\t0.079s" {
		t.Fatalf("evidence %q", got.Evidence)
	}
	if tail := decideCheck("PASS", out, "", 0, false); tail.Evidence != "PASS | ok  \texample.com/hello/strutil\t0.079s" {
		t.Fatalf("match already in the tail: %q", tail.Evidence)
	}
}
