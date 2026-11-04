// Package toolconformance defines the outbound port for tool-conformance
// fixture and state files.
package toolconformance

import contract "issueops/internal/contract/toolconformance"

// FixtureFiles is the outbound port for tool-conformance fixture storage: it
// reads the fixture manifest and regression fixtures, and initializes,
// writes, and reads per-run state files.
type FixtureFiles interface {
	ReadManifest() (contract.FixtureManifest, error)
	ReadRegression(string) (contract.RegressionFixture, error)
	InitializeState(string, []byte) error
	WriteState(string, []byte) error
	ReadState(string) ([]byte, error)
}
