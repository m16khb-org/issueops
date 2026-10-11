package toolconformance

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/toolconformance"
)

func evidenceID(ch string) string { return strings.Repeat(ch, 64) }

func validRegressionFixture() contract.RegressionFixture {
	diagnostics := []contract.Diagnostic{
		{Path: "$.a", Code: "unknown_key", Expected: "", Actual: "x"},
		{Path: "$.b", Code: "unknown_key", Expected: "", Actual: "y"},
	}
	return contract.RegressionFixture{
		SchemaVersion: 1, FixtureID: "fx", SourceTool: "src", ProbeTool: "probe",
		SourceSchemaSHA256: evidenceID("a"), Host: "codex", HostVersion: "1", ModelLabel: "m",
		CanonicalArguments: map[string]any{"k": "v"}, RawArgumentsSHA256: evidenceID("b"),
		ExpectedClassification: contract.UnknownKey, ExpectedDiagnostics: diagnostics,
		ExpectedDiagnosticSignature: DiagnosticSignature(contract.UnknownKey, diagnostics),
		ConfirmedEvidenceIDs:        []string{evidenceID("c"), evidenceID("d")},
		ExpectedFinalResult:         InvalidToolArgumentsResult("src", diagnostics),
		ExpectedStateUnchanged:      true,
	}
}

func TestValidateRegressionFixtureRejectsEachBrokenInvariant(t *testing.T) {
	if err := ValidateRegressionFixture(validRegressionFixture()); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*contract.RegressionFixture)
		want   string
	}{
		{"version", func(f *contract.RegressionFixture) { f.SchemaVersion = 2 }, "unsupported_regression_schema_version:2"},
		{"missing host", func(f *contract.RegressionFixture) { f.Host = "" }, "invalid_regression_fixture"},
		{"missing canonical args", func(f *contract.RegressionFixture) { f.CanonicalArguments = nil }, "invalid_regression_fixture"},
		{"bad schema digest", func(f *contract.RegressionFixture) { f.SourceSchemaSHA256 = "xyz" }, "invalid_regression_fixture_digest"},
		{"bad raw digest", func(f *contract.RegressionFixture) { f.RawArgumentsSHA256 = evidenceID("z") }, "invalid_regression_fixture_digest"},
		{"not drift", func(f *contract.RegressionFixture) { f.ExpectedClassification = contract.Classification("valid") }, "regression_classification_not_drift"},
		{"signature", func(f *contract.RegressionFixture) { f.ExpectedDiagnosticSignature = evidenceID("0") }, "regression_diagnostic_signature_mismatch"},
		{"unsorted diagnostics", func(f *contract.RegressionFixture) {
			f.ExpectedDiagnostics[0], f.ExpectedDiagnostics[1] = f.ExpectedDiagnostics[1], f.ExpectedDiagnostics[0]
		}, "regression_diagnostic_signature_mismatch"},
		{"bad evidence id", func(f *contract.RegressionFixture) { f.ConfirmedEvidenceIDs = []string{evidenceID("c"), "short"} }, "invalid_regression_evidence_id"},
		{"duplicate evidence", func(f *contract.RegressionFixture) {
			f.ConfirmedEvidenceIDs = []string{evidenceID("c"), evidenceID("c")}
		}, "regression_not_confirmed"},
		{"handler called", func(f *contract.RegressionFixture) { f.ExpectedHandlerCallCount = 1 }, "invalid_regression_behavioral_expectation"},
		{"state changed", func(f *contract.RegressionFixture) { f.ExpectedStateUnchanged = false }, "invalid_regression_behavioral_expectation"},
		{"final result", func(f *contract.RegressionFixture) { f.ExpectedFinalResult = map[string]any{"ok": true} }, "invalid_regression_final_result"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validRegressionFixture()
			f.ExpectedDiagnostics = append([]contract.Diagnostic(nil), f.ExpectedDiagnostics...)
			tc.mutate(&f)
			err := ValidateRegressionFixture(f)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

func TestInvalidToolArgumentsResultCopiesDiagnostics(t *testing.T) {
	in := []contract.Diagnostic{{Path: "$", Code: "c"}}
	out := InvalidToolArgumentsResult("tool", in)
	in[0].Code = "mutated"
	errBody := out["error"].(map[string]any)
	got := errBody["diagnostics"].([]contract.Diagnostic)
	if out["ok"] != false || out["isError"] != true || errBody["code"] != "invalid_tool_arguments" || errBody["tool"] != "tool" {
		t.Fatalf("unexpected envelope: %#v", out)
	}
	if got[0].Code != "c" {
		t.Fatalf("diagnostics alias caller slice: %#v", got)
	}
}

func TestRegressionDescriptorAndSource(t *testing.T) {
	schema := map[string]any{"type": "object"}
	sha, err := CanonicalSchemaSHA256(schema)
	if err != nil {
		t.Fatal(err)
	}
	fixture := validRegressionFixture()
	fixture.SourceSchemaSHA256 = sha
	descriptors := []contract.ToolDescriptor{{Name: "other"}, {Name: "src", InputSchema: schema}}

	descriptor, err := RegressionDescriptor(fixture, descriptors)
	if err != nil || descriptor.Name != "src" {
		t.Fatalf("descriptor = %v, %v", descriptor, err)
	}
	if _, err := RegressionDescriptor(fixture, descriptors[:1]); err == nil || err.Error() != "source_tool_not_found:src" {
		t.Fatalf("missing tool error = %v", err)
	}

	source := contract.Fixture{ID: "fx", SourceTool: "src", ProbeTool: "probe", SchemaSHA256: sha}
	got, err := RegressionSource(fixture, descriptor, []contract.Fixture{{ID: "zzz"}, source})
	if err != nil || got.ID != "fx" {
		t.Fatalf("source = %v, %v", got, err)
	}
	mismatches := map[string][]contract.Fixture{
		"regression_fixture_identity_mismatch": {{ID: "fx", SourceTool: "src", ProbeTool: "different", SchemaSHA256: sha}},
		"source_schema_hash_mismatch:src":      {{ID: "fx", SourceTool: "src", ProbeTool: "probe", SchemaSHA256: evidenceID("9")}},
	}
	for want, fixtures := range mismatches {
		if _, err := RegressionSource(fixture, descriptor, fixtures); err == nil || err.Error() != want {
			t.Fatalf("got %v, want %s", err, want)
		}
	}
	fixture.SourceSchemaSHA256 = evidenceID("8")
	if _, err := RegressionSource(fixture, descriptor, []contract.Fixture{source}); err == nil || err.Error() != "source_schema_hash_mismatch:src" {
		t.Fatalf("fixture hash mismatch = %v", err)
	}
}

func TestReplayMatchesRequiresEveryExpectation(t *testing.T) {
	fixture := validRegressionFixture()
	replay := func() contract.ReplayResult {
		return contract.ReplayResult{
			Classification: fixture.ExpectedClassification, DiagnosticSignature: fixture.ExpectedDiagnosticSignature,
			FinalResult: fixture.ExpectedFinalResult, StateBeforeSHA256: "s", StateAfterSHA256: "s",
		}
	}
	if !ReplayMatches(fixture, replay()) {
		t.Fatal("identical replay rejected")
	}
	mutations := map[string]func(*contract.ReplayResult){
		"classification": func(r *contract.ReplayResult) { r.Classification = contract.EnumMismatch },
		"signature":      func(r *contract.ReplayResult) { r.DiagnosticSignature = "other" },
		"handler calls":  func(r *contract.ReplayResult) { r.HandlerCalls = 1 },
		"state changed":  func(r *contract.ReplayResult) { r.StateAfterSHA256 = "t" },
		"final result":   func(r *contract.ReplayResult) { r.FinalResult = map[string]any{} },
	}
	for name, mutate := range mutations {
		r := replay()
		mutate(&r)
		if ReplayMatches(fixture, r) {
			t.Errorf("%s mismatch accepted", name)
		}
	}
}

func TestReplayMatchesComparesJSONNumbersByValue(t *testing.T) {
	fixture := validRegressionFixture()
	fixture.ExpectedFinalResult = map[string]any{"n": float64(1)}
	replay := contract.ReplayResult{
		Classification: fixture.ExpectedClassification, DiagnosticSignature: fixture.ExpectedDiagnosticSignature,
		FinalResult: map[string]any{"n": 1},
	}
	if !ReplayMatches(fixture, replay) {
		t.Fatal("int 1 and float 1 should compare equal after JSON normalization")
	}
	if jsonDeepEqual(func() {}, 1) {
		t.Fatal("unmarshalable values must not compare equal")
	}
}

func TestDiagnosticSignatureDependsOnClassificationAndDiagnostics(t *testing.T) {
	d := []contract.Diagnostic{{Path: "$", Code: "c"}}
	base := DiagnosticSignature(contract.UnknownKey, d)
	if !ValidEvidenceID(base) {
		t.Fatalf("signature is not a sha256 hex: %q", base)
	}
	if base == DiagnosticSignature(contract.EnumMismatch, d) || base == DiagnosticSignature(contract.UnknownKey, nil) {
		t.Fatal("signature ignores an input")
	}
	if base != DiagnosticSignature(contract.UnknownKey, d) {
		t.Fatal("signature is not deterministic")
	}
}

func TestValidEvidenceID(t *testing.T) {
	for value, want := range map[string]bool{evidenceID("a"): true, evidenceID("g"): false, "abcd": false, "": false} {
		if got := ValidEvidenceID(value); got != want {
			t.Errorf("ValidEvidenceID(%q) = %v", value, got)
		}
	}
}

func TestEpisodeCountsByFixture(t *testing.T) {
	episodes := []contract.EpisodeReport{
		{FixtureID: "a", Status: contract.EpisodeCompleted},
		{FixtureID: "a", Status: contract.EpisodeIncomplete},
		{FixtureID: "b", Status: contract.EpisodeCompleted},
	}
	if got := CompletedForFixture(episodes, "a"); got != 1 {
		t.Errorf("CompletedForFixture = %d", got)
	}
	if got := AttemptsForFixture(episodes, "a"); got != 2 {
		t.Errorf("AttemptsForFixture = %d", got)
	}
	if got := CountCompleted(episodes); got != 2 {
		t.Errorf("CountCompleted = %d", got)
	}
}

func TestValidatePreviousSelectionRejectsUnselectedHostOrFixture(t *testing.T) {
	selected := []SelectedFixturePair{{Host: "codex", Fixture: contract.Fixture{ID: "f1"}}}
	if err := ValidatePreviousSelection(nil, selected); err != nil {
		t.Fatalf("nil previous: %v", err)
	}
	ok := &contract.BenchmarkReport{Hosts: []contract.HostReport{{Host: "codex", Cases: []contract.EpisodeReport{{FixtureID: "f1"}}}}}
	if err := ValidatePreviousSelection(ok, selected); err != nil {
		t.Fatalf("valid selection: %v", err)
	}
	bad := map[string]*contract.BenchmarkReport{
		"empty host":      {Hosts: []contract.HostReport{{}}},
		"unselected host": {Hosts: []contract.HostReport{{Host: "claude"}}},
		"unselected pair": {Hosts: []contract.HostReport{{Host: "codex", Cases: []contract.EpisodeReport{{FixtureID: "f2"}}}}},
	}
	for name, previous := range bad {
		if err := ValidatePreviousSelection(previous, selected); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	fixture, found := SelectedFixtureForPair(selected, "codex", "f1")
	if !found || fixture.ID != "f1" {
		t.Fatalf("SelectedFixtureForPair = %v %v", fixture, found)
	}
	if _, found := SelectedFixtureForPair(selected, "claude", "f1"); found {
		t.Fatal("pair for another host found")
	}
}
