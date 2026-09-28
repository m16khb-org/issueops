package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
)

// Persisted lease records must pass both the strict shape codec and the domain
// invariants. These are the reviewed conversion boundaries that perform both.
func TestPersistedLeaseCodecHasNoUnreviewedBypass(t *testing.T) {
	root := findRepoRoot(t)
	allowed := map[string][]string{
		"internal/adapter/outbound/issueopsrecord/lease_codec.go": {"Decode", "Encode"},
		"cmd/issueops/issueopsapp/issueops_reconcile_wiring.go":   {"Decode"},
		"cmd/issueops/issueopsapp/issueops_resume_wiring.go":      {"Decode"},
	}
	actual := map[string][]string{}
	for _, source := range collectDDDInventory(t, root).Sources {
		path := filepath.Join(root, source.Path)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports in %s: %v", source.Path, err)
		}
		aliases := map[string]bool{}
		for _, imported := range file.Imports {
			name, err := strconv.Unquote(imported.Path.Value)
			if err != nil || name != "issueops/internal/contract/issueopslease" {
				continue
			}
			alias := "issueopslease"
			if imported.Name != nil {
				alias = imported.Name.Name
			}
			aliases[alias] = true
		}
		if len(aliases) == 0 {
			continue
		}
		file, err = parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", source.Path, err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (selector.Sel.Name != "Decode" && selector.Sel.Name != "Encode") {
				return true
			}
			alias, ok := selector.X.(*ast.Ident)
			if ok && aliases[alias.Name] {
				actual[source.Path] = append(actual[source.Path], selector.Sel.Name)
			}
			return true
		})
	}
	if !reflect.DeepEqual(actual, allowed) {
		t.Errorf("lease shape codec call sites changed; review domain validation at every site: got %v, want %v", actual, allowed)
	}
}
