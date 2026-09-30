package toolconformance

import (
	_ "embed"
	"encoding/json"
	contract "issueops/internal/contract/toolconformance"
)

//go:embed testdata/fixture_manifest.json
var manifestJSON []byte

type FixtureFiles struct{}

func (FixtureFiles) ReadManifest() (contract.FixtureManifest, error) {
	var manifest contract.FixtureManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return contract.FixtureManifest{}, err
	}
	return manifest, nil
}
