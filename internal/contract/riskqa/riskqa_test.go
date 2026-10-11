package riskqa

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizePaths(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"nil input yields empty non-nil slice", nil, []string{}},
		{"drops empty strings", []string{"", ""}, []string{}},
		{"dedupes and sorts", []string{"b.go", "a.go", "b.go"}, []string{"a.go", "b.go"}},
		{"sort is bytewise", []string{"b/x", "B/x", "a"}, []string{"B/x", "a", "b/x"}},
		{"keeps distinct spellings", []string{"./a.go", "a.go"}, []string{"./a.go", "a.go"}},
		{"slash paths unchanged", []string{"internal/x/y.go"}, []string{"internal/x/y.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePaths(tt.in)
			if got == nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("NormalizePaths(%q) = %#v, want %#v", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizePathsConvertsOSSeparators(t *testing.T) {
	native := filepath.Join("a", "b.go")
	got := NormalizePaths([]string{native, "a/b.go"})
	if !reflect.DeepEqual(got, []string{"a/b.go"}) {
		t.Fatalf("got %#v, want single slash-form path", got)
	}
}

func TestNormalizePathsDoesNotMutateInput(t *testing.T) {
	in := []string{"b", "a", "b"}
	NormalizePaths(in)
	if !reflect.DeepEqual(in, []string{"b", "a", "b"}) {
		t.Fatalf("input mutated: %#v", in)
	}
}

func TestRiskQATierPlanJSON(t *testing.T) {
	plan := RiskQATierPlan{
		Tier:         "high",
		ChangedPaths: []string{"a.go"},
		Reasons:      []string{"r"},
		Commands:     []string{"go test ./..."},
	}
	raw, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["scope"]; ok {
		t.Errorf("nil scope must be omitted: %s", raw)
	}
	for _, k := range []string{"tier", "changed_paths", "reasons", "commands"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing key %q in %s", k, raw)
		}
	}

	plan.Scope = &Scope{BaseSHA: "abc", Error: "boom"}
	raw, _ = json.Marshal(plan)
	var back RiskQATierPlan
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, plan) {
		t.Fatalf("round trip mismatch: %#v vs %#v", back, plan)
	}
	var scope map[string]any
	rawScope, _ := json.Marshal(plan.Scope)
	_ = json.Unmarshal(rawScope, &scope)
	if _, ok := scope["head_sha"]; ok {
		t.Errorf("empty head_sha must be omitted: %s", rawScope)
	}
}
