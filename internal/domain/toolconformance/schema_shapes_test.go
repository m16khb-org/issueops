package toolconformance

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/toolconformance"
)

func TestValidateRejectsMalformedSchemaKeywordShapes(t *testing.T) {
	for name, tt := range map[string]struct {
		schema map[string]any
		want   string
	}{
		"items not object":           {map[string]any{"items": "x"}, "invalid_schema_keyword_shape: items"},
		"additionalProperties":       {map[string]any{"additionalProperties": "no"}, "invalid_schema_keyword_shape: additionalProperties"},
		"required not strings":       {map[string]any{"required": []any{1}}, "invalid_schema_keyword_shape: required"},
		"required not list":          {map[string]any{"required": "a"}, "invalid_schema_keyword_shape: required"},
		"minimum not numeric":        {map[string]any{"minimum": "1"}, "invalid_schema_keyword_shape: minimum"},
		"maxLength negative":         {map[string]any{"maxLength": -1}, "invalid_schema_keyword_shape: maxLength"},
		"maxLength fractional":       {map[string]any{"maxLength": 1.5}, "invalid_schema_keyword_shape: maxLength"},
		"maxLength not numeric":      {map[string]any{"maxLength": "3"}, "invalid_schema_keyword_shape: maxLength"},
		"pattern not string":         {map[string]any{"pattern": 1}, "invalid_schema_keyword_shape: pattern"},
		"pattern uncompilable":       {map[string]any{"pattern": "("}, "invalid_schema_keyword_value: pattern"},
		"properties not object":      {map[string]any{"properties": []any{}}, "invalid_schema_keyword_shape: properties"},
		"property schema not object": {map[string]any{"properties": map[string]any{"a": 1}}, "invalid_schema_keyword_shape: properties/a"},
		"nested unsupported keyword": {map[string]any{"properties": map[string]any{"a": map[string]any{"oneOf": []any{}}}}, "schema keyword is not supported: oneOf"},
		"items nested unsupported":   {map[string]any{"items": map[string]any{"format": "uri"}}, "schema keyword is not supported: format"},
	} {
		diagnostics, err := Validate(tt.schema, nil)
		if err == nil || err.Error() != tt.want || diagnostics != nil {
			t.Errorf("%s: diagnostics=%v err=%v want %q", name, diagnostics, err, tt.want)
		}
	}
}

func TestValidateAcceptsAllSupportedKeywordValueForms(t *testing.T) {
	schema := map[string]any{
		"type": "object", "description": "d", "required": []string{"a"}, "additionalProperties": false,
		"properties": map[string]any{
			"a": map[string]any{"type": "string", "maxLength": int64(3), "pattern": "^x", "enum": []any{"xy"}},
			"n": map[string]any{"type": "number", "minimum": uint64(1)},
		},
	}
	diagnostics, err := Validate(schema, map[string]any{"a": "xy", "n": 2.0})
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v err=%v", diagnostics, err)
	}
}

func TestValidateReportsScalarTypeMismatchesWithActualType(t *testing.T) {
	for name, tt := range map[string]struct {
		typ    string
		value  any
		actual string
	}{
		"boolean got string": {"boolean", "true", "string"},
		"string got number":  {"string", 1.0, "number"},
		"number got string":  {"number", "1", "string"},
		"number got bool":    {"number", true, "boolean"},
		"integer got float":  {"integer", 1.5, "number"},
		"integer got array":  {"integer", []any{}, "array"},
		"string got object":  {"string", map[string]any{}, "object"},
		"string got nil":     {"string", nil, "unknown"},
	} {
		diagnostics, err := Validate(map[string]any{"type": tt.typ}, tt.value)
		want := contract.Diagnostic{Code: "wrong_type", Expected: tt.typ, Actual: tt.actual}
		if err != nil || len(diagnostics) != 1 || diagnostics[0] != want {
			t.Errorf("%s: diagnostics=%v err=%v want %v", name, diagnostics, err, want)
		}
	}
	for name, tt := range map[string]struct {
		typ   string
		value any
	}{"number accepts float": {"number", 1.5}, "integer accepts whole float": {"integer", 3.0}, "unknown type passes": {"null", "x"}} {
		diagnostics, err := Validate(map[string]any{"type": tt.typ}, tt.value)
		if err != nil || len(diagnostics) != 0 {
			t.Errorf("%s: diagnostics=%v err=%v", name, diagnostics, err)
		}
	}
}

func TestCanonicalSchemaSHA256RejectsUnencodableSchemaAndMatchesGoldenDigest(t *testing.T) {
	if _, err := CanonicalSchemaSHA256(map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("unencodable schema produced a digest")
	}
	const golden = "b152584da5cf35a8bb5e598655d62fcc6d04b5a4167188a178dcc45bdeed3bff" // sha256 of "{\"x\":1,\"y\":2}\n"
	got, err := CanonicalSchemaSHA256(map[string]any{"x": 1, "y": 2})
	if err != nil || got != golden {
		t.Fatalf("digest = %q err %v, want %s", got, err, golden)
	}
	if other, _ := CanonicalSchemaSHA256(map[string]any{"x": 1, "y": 3}); other == got {
		t.Fatal("digest ignores schema content")
	}
}

func TestPrepareManifestRejectsInvalidManifests(t *testing.T) {
	schema := map[string]any{"type": "object"}
	hash, _ := CanonicalSchemaSHA256(schema)
	descriptors := []contract.ToolDescriptor{{Name: "source", InputSchema: schema}}
	baseline := func(id string, n int) []contract.BaselineCase {
		cases := make([]contract.BaselineCase, n)
		for i := range cases {
			cases[i].FixtureID = id
		}
		return cases
	}
	fixture := contract.Fixture{ID: "f", SourceTool: "source", SchemaSHA256: hash}
	for name, tt := range map[string]struct {
		manifest    contract.FixtureManifest
		descriptors []contract.ToolDescriptor
		want        string
	}{
		"version":                   {contract.FixtureManifest{SchemaVersion: 99}, descriptors, "unsupported fixture manifest version 99"},
		"too few cases":             {contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{fixture}, BaselineCases: baseline("f", 9)}, descriptors, "baseline case count = 9, want 10"},
		"unknown fixture":           {contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{fixture}, BaselineCases: baseline("ghost", 10)}, descriptors, "references unknown fixture ghost"},
		"missing source":            {contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{fixture}, BaselineCases: baseline("f", 10)}, nil, "source tool not found: source"},
		"hash mismatch":             {contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{{ID: "f", SourceTool: "source", SchemaSHA256: "stale"}}, BaselineCases: baseline("f", 10)}, descriptors, "schema hash mismatch"},
		"unencodable source schema": {contract.FixtureManifest{SchemaVersion: contract.FixtureManifestVersion, Fixtures: []contract.Fixture{fixture}, BaselineCases: baseline("f", 10)}, []contract.ToolDescriptor{{Name: "source", InputSchema: map[string]any{"c": make(chan int)}}}, "unsupported type"},
	} {
		fixtures, cases, err := PrepareManifest(tt.manifest, tt.descriptors)
		if err == nil || !strings.Contains(err.Error(), tt.want) || fixtures != nil || cases != nil {
			t.Errorf("%s: err=%v", name, err)
		}
	}
}
