package quality

import (
	"go/ast"
	"go/parser"
	"go/token"
	contract "issueops/internal/contract/quality"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func CollectBranchFunctions(root string) ([]contract.BranchFunction, []string) {
	functions := []contract.BranchFunction{}
	warnings := []string{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, "branch scan: "+err.Error())
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".codegraph", ".issueops-runtime", "bin", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			warnings = append(warnings, "branch scan "+path+": "+err.Error())
			return nil
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			branches := countBranches(fn.Body)
			pos := fset.Position(fn.Pos())
			functions = append(functions, contract.BranchFunction{
				File:     relOrAbs(root, path),
				Line:     pos.Line,
				Name:     fn.Name.Name,
				Branches: branches,
			})
		}
		return nil
	})
	if err != nil {
		warnings = append(warnings, "branch scan: "+err.Error())
	}
	sort.Slice(functions, func(i, j int) bool {
		if functions[i].Branches != functions[j].Branches {
			return functions[i].Branches > functions[j].Branches
		}
		if functions[i].File != functions[j].File {
			return functions[i].File < functions[j].File
		}
		return functions[i].Line < functions[j].Line
	})
	return functions, warnings
}

func countBranches(node ast.Node) int {
	branches := 0
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.CaseClause, *ast.CommClause:
			branches++
		}
		return true
	})
	return branches
}

func CollectAuditItems(root string) ([]contract.AuditItem, []string) {
	path := filepath.Join(root, ".issueops", "PROJECT_AUDIT.md")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []string{"audit scan: " + err.Error()}
	}
	items := []contract.AuditItem{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") {
			continue
		}
		parts := splitMarkdownRow(line)
		if len(parts) < 5 || parts[0] == "ID" {
			continue
		}
		priority := strings.TrimSpace(parts[3])
		if priority != "P0" && priority != "P1" && priority != "P2" {
			continue
		}
		items = append(items, contract.AuditItem{
			ID:       strings.TrimSpace(parts[0]),
			Area:     strings.TrimSpace(parts[1]),
			Title:    strings.TrimSpace(parts[2]),
			Priority: priority,
			Size:     strings.TrimSpace(parts[4]),
		})
	}
	return items, nil
}

func splitMarkdownRow(line string) []string {
	line = strings.Trim(line, "|")
	raw := strings.Split(line, "|")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func relOrAbs(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return rel
}
