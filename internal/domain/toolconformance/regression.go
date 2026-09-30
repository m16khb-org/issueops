package toolconformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	contract "issueops/internal/contract/toolconformance"
	"reflect"
)

func ValidateRegressionFixture(fixture contract.RegressionFixture) error {
	if fixture.SchemaVersion != 1 {
		return fmt.Errorf("unsupported_regression_schema_version:%d", fixture.SchemaVersion)
	}
	if fixture.FixtureID == "" || fixture.SourceTool == "" || fixture.ProbeTool == "" || fixture.Host == "" ||
		fixture.HostVersion == "" || fixture.ModelLabel == "" || fixture.CanonicalArguments == nil {
		return fmt.Errorf("invalid_regression_fixture")
	}
	if !ValidEvidenceID(fixture.SourceSchemaSHA256) || !ValidEvidenceID(fixture.RawArgumentsSHA256) {
		return fmt.Errorf("invalid_regression_fixture_digest")
	}
	if !SchemaDriftClassification(fixture.ExpectedClassification) {
		return fmt.Errorf("regression_classification_not_drift")
	}
	diagnostics := append([]contract.Diagnostic(nil), fixture.ExpectedDiagnostics...)
	SortDiagnostics(diagnostics)
	if !jsonDeepEqual(diagnostics, fixture.ExpectedDiagnostics) ||
		fixture.ExpectedDiagnosticSignature != DiagnosticSignature(fixture.ExpectedClassification, diagnostics) {
		return fmt.Errorf("regression_diagnostic_signature_mismatch")
	}
	distinctEvidence := map[string]bool{}
	for _, id := range fixture.ConfirmedEvidenceIDs {
		if !ValidEvidenceID(id) {
			return fmt.Errorf("invalid_regression_evidence_id")
		}
		distinctEvidence[id] = true
	}
	if len(distinctEvidence) < 2 {
		return fmt.Errorf("regression_not_confirmed")
	}
	if fixture.ExpectedHandlerCallCount != 0 || !fixture.ExpectedStateUnchanged {
		return fmt.Errorf("invalid_regression_behavioral_expectation")
	}
	expectedFinal := InvalidToolArgumentsResult(fixture.SourceTool, diagnostics)
	if !jsonDeepEqual(fixture.ExpectedFinalResult, expectedFinal) {
		return fmt.Errorf("invalid_regression_final_result")
	}
	return nil
}

func RegressionDescriptor(fixture contract.RegressionFixture, descriptors []contract.ToolDescriptor) (contract.ToolDescriptor, error) {
	for _, descriptor := range descriptors {
		if descriptor.Name == fixture.SourceTool {
			return descriptor, nil
		}
	}
	return contract.ToolDescriptor{}, fmt.Errorf("source_tool_not_found:%s", fixture.SourceTool)
}

func RegressionSource(fixture contract.RegressionFixture, descriptor contract.ToolDescriptor, fixtures []contract.Fixture) (contract.Fixture, error) {
	var sourceFixture *contract.Fixture
	for index := range fixtures {
		if fixtures[index].ID == fixture.FixtureID {
			sourceFixture = &fixtures[index]
			break
		}
	}
	if sourceFixture == nil || sourceFixture.SourceTool != fixture.SourceTool || sourceFixture.ProbeTool != fixture.ProbeTool {
		return contract.Fixture{}, fmt.Errorf("regression_fixture_identity_mismatch")
	}
	actualSHA, err := CanonicalSchemaSHA256(descriptor.InputSchema)
	if err != nil {
		return contract.Fixture{}, err
	}
	if actualSHA != fixture.SourceSchemaSHA256 || actualSHA != sourceFixture.SchemaSHA256 {
		return contract.Fixture{}, fmt.Errorf("source_schema_hash_mismatch:%s", fixture.SourceTool)
	}
	return *sourceFixture, nil
}

func ReplayMatches(fixture contract.RegressionFixture, replay contract.ReplayResult) bool {
	return replay.Classification == fixture.ExpectedClassification &&
		replay.DiagnosticSignature == fixture.ExpectedDiagnosticSignature &&
		replay.HandlerCalls == fixture.ExpectedHandlerCallCount &&
		(!fixture.ExpectedStateUnchanged || replay.StateBeforeSHA256 == replay.StateAfterSHA256) &&
		jsonDeepEqual(replay.FinalResult, fixture.ExpectedFinalResult)
}

func jsonDeepEqual(left, right any) bool {
	var normalized [2]any
	for i, value := range []any{left, right} {
		data, err := json.Marshal(value)
		if err != nil {
			return false
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&normalized[i]); err != nil {
			return false
		}
	}
	return reflect.DeepEqual(normalized[0], normalized[1])
}
