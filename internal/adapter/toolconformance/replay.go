package toolconformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	contract "issueops/internal/contract/toolconformance"
	"os"
	"path/filepath"
)

const regressionFixtureLimit = 64 << 10

func (FixtureFiles) ReadRegression(path string) (contract.RegressionFixture, error) {
	file, err := os.Open(path)
	if err != nil {
		return contract.RegressionFixture{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, regressionFixtureLimit+1))
	if err != nil {
		return contract.RegressionFixture{}, err
	}
	if len(data) > regressionFixtureLimit {
		return contract.RegressionFixture{}, fmt.Errorf("regression_fixture_too_large")
	}
	var fixture contract.RegressionFixture
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return contract.RegressionFixture{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return contract.RegressionFixture{}, fmt.Errorf("regression_fixture_trailing_json")
	}
	return fixture, nil
}

func (files FixtureFiles) InitializeState(dir string, data []byte) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return files.WriteState(dir, data)
}
func (FixtureFiles) WriteState(dir string, data []byte) error {
	return os.WriteFile(filepath.Join(dir, "state.json"), data, 0o600)
}
func (FixtureFiles) ReadState(dir string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, "state.json"))
}
