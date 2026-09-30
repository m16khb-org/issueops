package toolconformance

import contract "issueops/internal/contract/toolconformance"

type FixtureFiles interface {
	ReadManifest() (contract.FixtureManifest, error)
	ReadRegression(string) (contract.RegressionFixture, error)
	InitializeState(string, []byte) error
	WriteState(string, []byte) error
	ReadState(string) ([]byte, error)
}
