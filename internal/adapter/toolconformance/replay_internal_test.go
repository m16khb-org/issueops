package toolconformance

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestReadRegressionEnforcesFixtureSizeLimitExactly(t *testing.T) {
	body := []byte("{}")
	atLimit := append(bytes.Repeat([]byte(" "), regressionFixtureLimit-len(body)), body...)
	overLimit := append([]byte(" "), atLimit...)

	dir := t.TempDir()
	atPath := filepath.Join(dir, "at.json")
	overPath := filepath.Join(dir, "over.json")
	if err := os.WriteFile(atPath, atLimit, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(overPath, overLimit, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := (FixtureFiles{}).ReadRegression(atPath); err != nil {
		t.Fatalf("fixture of exactly %d bytes must be accepted: %v", regressionFixtureLimit, err)
	}
	_, err := (FixtureFiles{}).ReadRegression(overPath)
	if err == nil || err.Error() != "regression_fixture_too_large" {
		t.Fatalf("fixture of %d bytes err=%v, want regression_fixture_too_large", regressionFixtureLimit+1, err)
	}
}
