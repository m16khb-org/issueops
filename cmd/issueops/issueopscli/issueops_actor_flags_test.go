package issueopscli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// actor flag를 등록만 하고 반환값을 버리면 명령은 --host·--session-id·--cwd를
// 받아 주지만 core에는 actor가 전달되지 않는다. 사용자는 fence가 적용됐다고
// 믿게 되고 core는 호출자를 확인하지 않는다. implementation-review,
// project-docs-review, schema-evidence record가 이 상태였다.
func TestActorFlagRegistrationsAreNeverDiscarded(t *testing.T) {
	registrars := map[string]bool{"addIssueOpsActorFlags": true, "addActorFlags": true}
	var discarded []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		file, err := parser.ParseFile(fileSet, path, nil, 0)
		if err != nil {
			return err
		}
		registrarCall := func(expr ast.Expr) (*ast.CallExpr, bool) {
			call, ok := expr.(*ast.CallExpr)
			if !ok {
				return nil, false
			}
			name, ok := call.Fun.(*ast.Ident)
			return call, ok && registrars[name.Name]
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch statement := node.(type) {
			case *ast.ExprStmt:
				if call, ok := registrarCall(statement.X); ok {
					discarded = append(discarded, fileSet.Position(call.Pos()).String())
				}
			case *ast.AssignStmt:
				// `_ = addIssueOpsActorFlags(fs)`도 반환값을 버린다.
				for index, rhs := range statement.Rhs {
					call, ok := registrarCall(rhs)
					if !ok || index >= len(statement.Lhs) {
						continue
					}
					if blank, ok := statement.Lhs[index].(*ast.Ident); ok && blank.Name == "_" {
						discarded = append(discarded, fileSet.Position(call.Pos()).String())
					}
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(discarded) != 0 {
		t.Fatalf("actor flags are registered but never passed to the core:\n%s", strings.Join(discarded, "\n"))
	}
}
