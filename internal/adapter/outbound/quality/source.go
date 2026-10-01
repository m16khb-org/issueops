package quality

import (
	"fmt"
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
	paths, scanErrors := productionGoFiles(root)
	for _, err := range scanErrors {
		warnings = append(warnings, "branch scan: "+err.Error())
	}
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			warnings = append(warnings, "branch scan "+path+": "+err.Error())
			continue
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
		return nil, []string{"audit scan: " + err.Error()}
	}
	items := []contract.AuditItem{}
	warnings := []string{}
	warn := func(line int, reason string) {
		warnings = append(warnings, fmt.Sprintf("audit scan %s:%d: %s", path, line, reason))
	}
	lines := strings.Split(string(b), "\n")
	hasHeadings := false
	for _, line := range lines {
		if level, _ := auditHeading(strings.TrimSpace(line)); level > 0 {
			hasHeadings = true
			break
		}
	}
	openLevel := 0
	var header map[string]int
	width := 0
	inTable, validTable, explicitNone := false, false, false
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if level, title := auditHeading(line); level > 0 {
			if openLevel > 0 && level <= openLevel {
				openLevel = 0
			}
			if title == "open" || strings.HasPrefix(title, "open ") {
				openLevel = level
			}
			inTable = false
			continue
		}
		if hasHeadings && openLevel == 0 {
			continue
		}
		if openLevel > 0 && line == "_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._" {
			explicitNone = true
		}
		if !strings.HasPrefix(line, "|") {
			if inTable && header != nil && strings.Contains(line, "|") {
				warn(i+1, "audit row is missing its leading pipe")
			}
			inTable = false
			continue
		}
		parts := splitMarkdownRow(line)
		if !inTable {
			inTable = true
			width = len(parts)
			header = auditColumns(parts)
			if header == nil {
				warn(i+1, "invalid audit header: require unique ID, Area, Title, Priority, Size columns")
				continue
			}
			if i+1 >= len(lines) || !auditSeparator(strings.TrimSpace(lines[i+1]), width) {
				warn(i+1, "missing or invalid audit table separator")
				header = nil
				continue
			}
			i++
			validTable = true
			continue
		}
		if header == nil {
			continue
		}
		if len(parts) != width {
			warn(i+1, "audit row width does not match header")
			continue
		}
		missing := false
		for _, name := range []string{"id", "area", "title", "priority", "size"} {
			if parts[header[name]] == "" {
				warn(i+1, "empty required audit cell: "+name)
				missing = true
			}
		}
		if missing {
			continue
		}
		priority := parts[header["priority"]]
		if priority != "P0" && priority != "P1" && priority != "P2" && priority != "P3" {
			warn(i+1, "invalid audit priority: require P0, P1, P2 or P3")
			continue
		}
		if priority == "P3" {
			continue
		}
		items = append(items, contract.AuditItem{
			ID:       parts[header["id"]],
			Area:     parts[header["area"]],
			Title:    parts[header["title"]],
			Priority: priority,
			Size:     parts[header["size"]],
		})
	}
	if !validTable && !explicitNone {
		warn(1, "no valid open audit table or explicit zero declaration")
	}
	if explicitNone && len(items) > 0 {
		warn(1, "explicit zero declaration contradicts open audit items")
	}
	return items, warnings
}

func auditHeading(line string) (int, string) {
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level == 0 || level > 6 || len(line) <= level || line[level] != ' ' {
		return 0, ""
	}
	return level, strings.ToLower(strings.TrimSpace(strings.TrimRight(line[level:], "#")))
}

func auditColumns(parts []string) map[string]int {
	columns := make(map[string]int, len(parts))
	for i, part := range parts {
		name := strings.ToLower(strings.TrimSpace(part))
		if _, exists := columns[name]; name == "" || exists {
			return nil
		}
		columns[name] = i
	}
	for _, name := range []string{"id", "area", "title", "priority", "size"} {
		if _, exists := columns[name]; !exists {
			return nil
		}
	}
	return columns
}

func auditSeparator(line string, width int) bool {
	if !strings.HasPrefix(line, "|") {
		return false
	}
	parts := splitMarkdownRow(line)
	if len(parts) != width {
		return false
	}
	for _, part := range parts {
		part = strings.TrimSuffix(strings.TrimPrefix(part, ":"), ":")
		if len(part) < 3 || strings.Trim(part, "-") != "" {
			return false
		}
	}
	return true
}

func splitMarkdownRow(line string) []string {
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	raw := strings.Split(line, "|")
	out := make([]string, 0, len(raw))
	for _, part := range raw {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func relOrAbs(root, path string) string {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(absoluteRoot, path)
	if err != nil {
		return path
	}
	return rel
}
