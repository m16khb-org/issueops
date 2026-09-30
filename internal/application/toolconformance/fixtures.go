package toolconformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	contract "issueops/internal/contract/toolconformance"
	domain "issueops/internal/domain/toolconformance"
	fixtureport "issueops/internal/port/toolconformance"
)

type FixtureService struct{ Files fixtureport.FixtureFiles }

func (s FixtureService) LoadManifest(descriptors []contract.ToolDescriptor) ([]contract.Fixture, []contract.BaselineCase, error) {
	manifest, err := s.Files.ReadManifest()
	if err != nil {
		return nil, nil, err
	}
	return domain.PrepareManifest(manifest, descriptors)
}

func (s FixtureService) LoadRegressionFixture(path string) (contract.RegressionFixture, error) {
	fixture, err := s.Files.ReadRegression(path)
	if err != nil {
		return contract.RegressionFixture{}, err
	}
	if err := domain.ValidateRegressionFixture(fixture); err != nil {
		return contract.RegressionFixture{}, err
	}
	return fixture, nil
}

func (s FixtureService) ReplayRegression(fixture contract.RegressionFixture, descriptors []contract.ToolDescriptor, stateDir string) (contract.ReplayResult, error) {
	if err := domain.ValidateRegressionFixture(fixture); err != nil {
		return contract.ReplayResult{}, err
	}
	descriptor, err := domain.RegressionDescriptor(fixture, descriptors)
	if err != nil {
		return contract.ReplayResult{}, err
	}
	fixtures, _, err := s.LoadManifest(descriptors)
	if err != nil {
		return contract.ReplayResult{}, err
	}
	source, err := domain.RegressionSource(fixture, descriptor, fixtures)
	if err != nil {
		return contract.ReplayResult{}, err
	}
	state := []byte(`{"stable":true}`)
	if err := s.Files.InitializeState(stateDir, state); err != nil {
		return contract.ReplayResult{}, err
	}
	before := sha256.Sum256(state)
	raw, err := json.Marshal(fixture.CanonicalArguments)
	if err != nil {
		return contract.ReplayResult{}, err
	}
	result, err := domain.Classify(contract.CallObservation{RawArguments: raw, CallCount: 1}, descriptor.InputSchema, source.ExpectedArguments)
	if err != nil {
		return contract.ReplayResult{}, err
	}
	handlerCalls := 0
	finalResult := domain.InvalidToolArgumentsResult(fixture.SourceTool, result.Diagnostics)
	if result.CanonicalValid {
		handlerCalls++
		// Preserve the probe handler's observed-state contract even if its write fails.
		_ = s.Files.WriteState(stateDir, []byte(`{"stable":false}`))
		finalResult = map[string]any{"ok": true}
	}
	afterState, err := s.Files.ReadState(stateDir)
	if err != nil {
		return contract.ReplayResult{}, err
	}
	after := sha256.Sum256(afterState)
	replay := contract.ReplayResult{Classification: result.Classification, Diagnostics: result.Diagnostics, DiagnosticSignature: domain.DiagnosticSignature(result.Classification, result.Diagnostics), HandlerCalls: handlerCalls, FinalResult: finalResult, StateBeforeSHA256: hex.EncodeToString(before[:]), StateAfterSHA256: hex.EncodeToString(after[:])}
	replay.OK = domain.ReplayMatches(fixture, replay)
	return replay, nil
}
