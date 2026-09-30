package toolconformance

import (
	"encoding/json"
	"fmt"
	contract "issueops/internal/contract/toolconformance"
	"sort"
)

func PrepareManifest(d contract.FixtureManifest, descriptors []contract.ToolDescriptor) ([]contract.Fixture, []contract.BaselineCase, error) {
	if d.SchemaVersion != contract.FixtureManifestVersion {
		return nil, nil, fmt.Errorf("unsupported fixture manifest version %d", d.SchemaVersion)
	}
	if err := validateBaselineCases(d.Fixtures, d.BaselineCases); err != nil {
		return nil, nil, err
	}
	m := map[string]contract.ToolDescriptor{}
	for _, v := range descriptors {
		m[v.Name] = v
	}
	for _, f := range d.Fixtures {
		tool, ok := m[f.SourceTool]
		if !ok {
			return nil, nil, fmt.Errorf("source tool not found: %s", f.SourceTool)
		}
		actualSHA, err := CanonicalSchemaSHA256(tool.InputSchema)
		if err != nil {
			return nil, nil, err
		}
		if f.SchemaSHA256 != actualSHA {
			return nil, nil, fmt.Errorf("fixture %s source tool %s schema hash mismatch: want %s got %s", f.ID, f.SourceTool, f.SchemaSHA256, actualSHA)
		}
	}
	d.Fixtures = append([]contract.Fixture(nil), d.Fixtures...)
	d.BaselineCases = append([]contract.BaselineCase(nil), d.BaselineCases...)
	sort.Slice(d.Fixtures, func(i, j int) bool { return d.Fixtures[i].ID < d.Fixtures[j].ID })
	for i := range d.Fixtures {
		d.Fixtures[i].ExpectedArguments = cloneArguments(d.Fixtures[i].ExpectedArguments)
	}
	for i := range d.BaselineCases {
		d.BaselineCases[i].Arguments = cloneArguments(d.BaselineCases[i].Arguments)
	}
	return d.Fixtures, d.BaselineCases, nil
}

func validateBaselineCases(fixtures []contract.Fixture, cases []contract.BaselineCase) error {
	if len(cases) != 10 {
		return fmt.Errorf("baseline case count = %d, want 10", len(cases))
	}
	known := map[string]bool{}
	for _, fixture := range fixtures {
		known[fixture.ID] = true
	}
	for i, baseline := range cases {
		if !known[baseline.FixtureID] {
			return fmt.Errorf("baseline case %d references unknown fixture %s", i, baseline.FixtureID)
		}
	}
	return nil
}

func cloneArguments(in map[string]any) map[string]any {
	b, _ := json.Marshal(in)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}
