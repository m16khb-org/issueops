package toolconformance

import (
	"errors"
	contract "issueops/internal/contract/toolconformance"
	domain "issueops/internal/domain/toolconformance"
	"reflect"
	"strings"
	"testing"
)

type fixtureFilesFake struct {
	readOverride                             []byte
	manifest                                 contract.FixtureManifest
	regression                               contract.RegressionFixture
	events                                   []string
	state                                    []byte
	manifestErr, resetErr, writeErr, readErr error
}

func (f *fixtureFilesFake) ReadManifest() (contract.FixtureManifest, error) {
	f.events = append(f.events, "manifest")
	return f.manifest, f.manifestErr
}
func (f *fixtureFilesFake) ReadRegression(string) (contract.RegressionFixture, error) {
	f.events = append(f.events, "regression")
	return f.regression, nil
}
func (f *fixtureFilesFake) InitializeState(_ string, data []byte) error {
	f.events = append(f.events, "reset")
	if f.resetErr != nil {
		return f.resetErr
	}
	f.state = append([]byte(nil), data...)
	return nil
}
func (f *fixtureFilesFake) WriteState(_ string, data []byte) error {
	f.events = append(f.events, "write")
	if f.writeErr != nil {
		return f.writeErr
	}
	f.state = append([]byte(nil), data...)
	return nil
}
func (f *fixtureFilesFake) ReadState(string) ([]byte, error) {
	f.events = append(f.events, "read")
	if f.readOverride != nil {
		return f.readOverride, f.readErr
	}
	return f.state, f.readErr
}

func regressionScenario(t *testing.T) (*fixtureFilesFake, contract.RegressionFixture, []contract.ToolDescriptor) {
	t.Helper()
	schema := map[string]any{"type": "object", "properties": map[string]any{}}
	hash, err := domain.CanonicalSchemaSHA256(schema)
	if err != nil {
		t.Fatal(err)
	}
	classified, err := domain.Classify(contract.CallObservation{CallCount: 1, RawArguments: []byte(`{"extra":true}`)}, schema, map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	regression := contract.RegressionFixture{SchemaVersion: 1, FixtureID: "f", SourceTool: "source", ProbeTool: "probe", SourceSchemaSHA256: hash, Host: "claude", HostVersion: "test", ModelLabel: "test", CanonicalArguments: map[string]any{"extra": true}, RawArgumentsSHA256: strings.Repeat("a", 64), ExpectedClassification: classified.Classification, ExpectedDiagnostics: classified.Diagnostics, ExpectedDiagnosticSignature: domain.DiagnosticSignature(classified.Classification, classified.Diagnostics), ConfirmedEvidenceIDs: []string{strings.Repeat("b", 64), strings.Repeat("c", 64)}, ExpectedHandlerCallCount: 0, ExpectedFinalResult: domain.InvalidToolArgumentsResult("source", classified.Diagnostics), ExpectedStateUnchanged: true}
	manifest := contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{{ID: "f", SourceTool: "source", ProbeTool: "probe", SchemaSHA256: hash, ExpectedArguments: map[string]any{}}}}
	for i := 0; i < 10; i++ {
		manifest.BaselineCases = append(manifest.BaselineCases, contract.BaselineCase{FixtureID: "f"})
	}
	return &fixtureFilesFake{manifest: manifest, regression: regression}, regression, []contract.ToolDescriptor{{Name: "source", InputSchema: schema}}
}

func TestRegressionReplayRefusesBeforeEffectsAndPreservesErrorOrder(t *testing.T) {
	failure := errors.New("storage failure")
	for _, tc := range []struct {
		name   string
		change func(*fixtureFilesFake, *contract.RegressionFixture, *[]contract.ToolDescriptor)
		want   string
		events []string
	}{
		{"invalid fixture", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			r.SchemaVersion = 2
			f.manifestErr = failure
		}, "unsupported_regression_schema_version:2", nil},
		{"missing descriptor", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			*d = nil
			f.manifestErr = failure
		}, "source_tool_not_found:source", nil},
		{"manifest read", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			f.manifestErr = failure
		}, "storage failure", []string{"manifest"}},
		{"source mismatch", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			r.ProbeTool = "other"
		}, "regression_fixture_identity_mismatch", []string{"manifest"}},
		{"schema mismatch", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			r.SourceSchemaSHA256 = strings.Repeat("f", 64)
		}, "source_schema_hash_mismatch:source", []string{"manifest"}},
		{"reset error", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			f.resetErr = failure
		}, "storage failure", []string{"manifest", "reset"}},
		{"marshal after reset", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			r.CanonicalArguments = func() {}
		}, "json: unsupported type", []string{"manifest", "reset"}},
		{"read error", func(f *fixtureFilesFake, r *contract.RegressionFixture, d *[]contract.ToolDescriptor) {
			f.readErr = failure
		}, "storage failure", []string{"manifest", "reset", "read"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, r, d := regressionScenario(t)
			tc.change(f, &r, &d)
			got, err := (FixtureService{Files: f}).ReplayRegression(r, d, "scratch")
			if err == nil || !strings.Contains(err.Error(), tc.want) || !reflect.DeepEqual(f.events, tc.events) || !reflect.DeepEqual(got, contract.ReplayResult{}) {
				t.Fatalf("result=%+v error=%v events=%v want=%v", got, err, f.events, tc.events)
			}
		})
	}
}

func TestRegressionReplayUsesCanonicalValidityForHandlerAndObservesState(t *testing.T) {
	for _, canonical := range []bool{false, true} {
		for _, failWrite := range []bool{false, true} {
			f, r, d := regressionScenario(t)
			if canonical {
				r.CanonicalArguments = map[string]any{}
			}
			if failWrite {
				f.writeErr = errors.New("handler write failed")
			}
			got, err := (FixtureService{Files: f}).ReplayRegression(r, d, "scratch")
			if err != nil {
				t.Fatal(err)
			}
			events := []string{"manifest", "reset"}
			if canonical {
				events = append(events, "write")
			}
			events = append(events, "read")
			wantCalls := 0
			if canonical {
				wantCalls = 1
			}
			if got.OK == canonical || got.HandlerCalls != wantCalls || !reflect.DeepEqual(events, f.events) {
				t.Fatalf("canonical=%v failedWrite=%v got=%+v events=%v", canonical, failWrite, got, f.events)
			}
			unchanged := got.StateBeforeSHA256 == got.StateAfterSHA256
			if unchanged != (!canonical || failWrite) {
				t.Fatalf("unexpected state hashes: %+v", got)
			}
		}
	}
}

func TestRegressionFixtureValidationAfterRead(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*contract.RegressionFixture)
		want   string
	}{
		{"distinct evidence", func(r *contract.RegressionFixture) { r.ConfirmedEvidenceIDs[1] = r.ConfirmedEvidenceIDs[0] }, "regression_not_confirmed"},
		{"invalid evidence", func(r *contract.RegressionFixture) { r.ConfirmedEvidenceIDs[0] = "bad" }, "invalid_regression_evidence_id"},
		{"handler expectation", func(r *contract.RegressionFixture) { r.ExpectedHandlerCallCount = 1 }, "invalid_regression_behavioral_expectation"},
		{"state expectation", func(r *contract.RegressionFixture) { r.ExpectedStateUnchanged = false }, "invalid_regression_behavioral_expectation"},
		{"final result", func(r *contract.RegressionFixture) { r.ExpectedFinalResult = map[string]any{"ok": true} }, "invalid_regression_final_result"},
		{"signature", func(r *contract.RegressionFixture) { r.ExpectedDiagnosticSignature = "wrong" }, "regression_diagnostic_signature_mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, r, _ := regressionScenario(t)
			tc.change(&r)
			f.regression = r
			got, err := (FixtureService{Files: f}).LoadRegressionFixture("fixture.json")
			if err == nil || err.Error() != tc.want || !reflect.DeepEqual(got, contract.RegressionFixture{}) || !reflect.DeepEqual(f.events, []string{"regression"}) {
				t.Fatalf("got=%+v err=%v events=%v", got, err, f.events)
			}
		})
	}
}

func TestRegressionReplayRejectsUnexpectedObservedStateChange(t *testing.T) {
	f, r, d := regressionScenario(t)
	f.readOverride = []byte(`{"external_change":true}`)
	got, err := (FixtureService{Files: f}).ReplayRegression(r, d, "scratch")
	if err != nil {
		t.Fatal(err)
	}
	if got.OK || got.HandlerCalls != 0 || got.StateBeforeSHA256 == got.StateAfterSHA256 {
		t.Fatalf("unobserved state mutation accepted: %+v", got)
	}
}
