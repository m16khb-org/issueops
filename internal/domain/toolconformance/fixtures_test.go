package toolconformance

import (
	"encoding/json"
	contract "issueops/internal/contract/toolconformance"
	"testing"
)

func TestPrepareManifestPreservesInputAndIsolatesNestedArguments(t *testing.T) {
	schema := map[string]any{"type": "object"}
	hash, err := CanonicalSchemaSHA256(schema)
	if err != nil {
		t.Fatal(err)
	}
	manifest := contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{
		{ID: "b", SourceTool: "source", SchemaSHA256: hash, ExpectedArguments: map[string]any{"nested": []any{"original"}}},
		{ID: "a", SourceTool: "source", SchemaSHA256: hash, ExpectedArguments: map[string]any{}},
	}}
	for i := 0; i < 10; i++ {
		manifest.BaselineCases = append(manifest.BaselineCases, contract.BaselineCase{FixtureID: "b", Arguments: map[string]any{"nested": []any{"original"}}})
	}
	before, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	fixtures, cases, err := PrepareManifest(manifest, []contract.ToolDescriptor{{Name: "source", InputSchema: schema}})
	if err != nil {
		t.Fatal(err)
	}
	if fixtures[0].ID != "a" || fixtures[1].ID != "b" {
		t.Fatalf("order=%+v", fixtures)
	}
	fixtures[1].ExpectedArguments["nested"].([]any)[0] = "changed"
	cases[0].Arguments["nested"].([]any)[0] = "changed"
	after, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("input mutated: %s -> %s", before, after)
	}
}
