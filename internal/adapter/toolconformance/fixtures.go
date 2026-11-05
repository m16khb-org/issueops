// Package toolconformance is the file adapter for tool conformance: it serves the
// embedded fixture manifest, regression fixtures read from disk, and the replay
// state file that conformance runs persist.
package toolconformance

import (
	_ "embed"
	"encoding/json"
	contract "issueops/internal/contract/toolconformance"
)

//go:embed testdata/fixture_manifest.json
var manifestJSON []byte

// FixtureFiles reads and writes the fixtures and replay state used by tool
// conformance runs. It holds no state.
type FixtureFiles struct{}

// ReadManifest decodes the fixture manifest embedded from
// testdata/fixture_manifest.json.
func (FixtureFiles) ReadManifest() (contract.FixtureManifest, error) {
	var manifest contract.FixtureManifest
	if err := json.Unmarshal(manifestJSON, &manifest); err != nil {
		return contract.FixtureManifest{}, err
	}
	return manifest, nil
}
