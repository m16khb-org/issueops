package gates

import (
	"strings"
	"testing"
)

func TestAppendAbandonInsertsBeforeTrailingBlankLinesAndShiftsLaterIndexes(t *testing.T) {
	ledger := Parse("- [ ] G1: a\n\n\r\n")
	AppendAbandon(&ledger, "G1", "  not needed ")
	if got := Render(ledger); got != "- [ ] G1: a\nABANDON: G1 not needed\n\n\r\n" {
		t.Fatalf("got %q", got)
	}
	if !ledger.Gates[0].Abandoned || ledger.Gates[0].AbandonReason != "not needed" || ledger.Gates[0].CheckboxLine != 0 {
		t.Fatalf("gate=%+v", ledger.Gates[0])
	}
	reparsed := Parse(Render(ledger))
	if !reparsed.Gates[0].Abandoned {
		t.Fatal("abandon line must be re-parseable")
	}
}

func TestAppendAbandonWithBlankReasonRecordsPlaceholder(t *testing.T) {
	ledger := Parse("- [ ] G1: a\n")
	AppendAbandon(&ledger, "G1", "  ")
	if ledger.Gates[0].AbandonReason != "(no reason)" {
		t.Fatalf("reason=%q", ledger.Gates[0].AbandonReason)
	}
}

func TestMarkPassInsertedEvidenceKeepsFollowingGateIndexesValid(t *testing.T) {
	ledger := Parse("- [ ] G1: a\n- [ ] G2: b\n  EVIDENCE: pending\n")
	MarkPass(&ledger, 0, "done")
	MarkPass(&ledger, 1, "also")
	got := Render(ledger)
	want := "- [x] G1: a\n  EVIDENCE: done\n- [x] G2: b\n  EVIDENCE: also\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMarkPassReusesIndentOfExistingAttributeLines(t *testing.T) {
	ledger := Parse("- [ ] G1: a\n\r\n# note\n\tCHECK: true\n")
	MarkPass(&ledger, 0, "ok")
	if !strings.Contains(Render(ledger), "\n\tEVIDENCE: ok\n") {
		t.Fatalf("indent not reused: %q", Render(ledger))
	}
	ledger = Parse("- [ ] G1: a\nprose line\n- [ ] G2: b\n  CHECK: true\n")
	MarkPass(&ledger, 0, "ok")
	if !strings.Contains(Render(ledger), "- [x] G1: a\n  EVIDENCE: ok\nprose") {
		t.Fatalf("prose must stop the indent search: %q", Render(ledger))
	}
}

func TestExpectMatchesIgnoresUnknownRegexFlagsAndInvalidPatterns(t *testing.T) {
	if !ExpectMatches("/^OK$/ims", "x\nok\ny") {
		t.Fatal("i and m flags must apply")
	}
	if !ExpectMatches("/a.b/s", "a\nb") {
		t.Fatal("s flag must let dot match newline")
	}
	if ExpectMatches("/a.b/", "a\nb") {
		t.Fatal("without s the dot must not match newline")
	}
	if !ExpectMatches("/ok/g", "ok") {
		t.Fatal("unsupported flags are ignored")
	}
	if ExpectMatches("/(/", "(") {
		t.Fatal("invalid regex must not match")
	}
	if !ExpectMatches("ok", "line one\r\nok\r\n") {
		t.Fatal("CRLF output must match line-anchored literal")
	}
}

func TestEvidenceTailDefaultsLimitAndReportsEmptyOutput(t *testing.T) {
	if got := EvidenceTail(" \n\r\n ", 0); got != "(no output)" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("x", 500)
	if got := EvidenceTail(long, -1); len(got) > 200 || len(got) == 0 {
		t.Fatalf("default limit not applied: len=%d", len(got))
	}
	if got := EvidenceTail("a\nb\nc", 100); got != "b | c" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckArgvSplitsQuotedTokensAndRejectsEmpty(t *testing.T) {
	argv, msg := CheckArgv(`go test "./a b"`)
	if msg != "" || len(argv) != 3 || argv[2] != "./a b" {
		t.Fatalf("argv=%q msg=%q", argv, msg)
	}
	if argv, msg := CheckArgv("   "); argv != nil || msg != "empty CHECK command" {
		t.Fatalf("argv=%q msg=%q", argv, msg)
	}
}

func TestGateFileSlugCapsAt64RunesWithoutTrailingDash(t *testing.T) {
	got := gateFileSlug(strings.Repeat("a", 63) + " -- b")
	if got != strings.Repeat("a", 63)+"-b" {
		t.Fatalf("got %q", got)
	}
	if got := gateFileSlug(strings.Repeat("가", 70)); len([]rune(got)) != 64 {
		t.Fatalf("runes=%d", len([]rune(got)))
	}
}

func TestRenderInitialLedgerEmitsOptionalAttributesInSpecOrder(t *testing.T) {
	spec := "G1: proof | CHECK: printf ok | EXPECT: ok | EVIDENCE: manual"
	out, err := renderGateSpec(spec)
	if err != nil {
		t.Fatal(err)
	}
	if out != "- [ ] G1: proof\n  CHECK: printf ok\n  EXPECT: ok\n  EVIDENCE: manual" {
		t.Fatalf("out=%q", out)
	}
}

func TestGateFileSlugDropsLeadingSeparators(t *testing.T) {
	if got := gateFileSlug("--Hello  World"); got != "hello-world" {
		t.Fatalf("got %q", got)
	}
}

// An exit-code-only gate (empty EXPECT) leaves no trailing space for
// `git diff --check` to reject in the committed ledger.
func TestRenderGateSpecEmptyExpectHasNoTrailingSpace(t *testing.T) {
	out, err := renderGateSpec("G1: parses | CHECK: sh -n a.sh | EXPECT:")
	if err != nil {
		t.Fatal(err)
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasSuffix(line, " ") {
			t.Fatalf("trailing space in %q", line)
		}
	}
	if !strings.Contains(out, "\n  EXPECT:\n") {
		t.Fatalf("ledger = %q", out)
	}
}
