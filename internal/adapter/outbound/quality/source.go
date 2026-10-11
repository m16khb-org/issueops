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

// auditNoneDeclaration은 open 섹션에 열린 항목이 없음을 명시하는 문장이다.
const auditNoneDeclaration = "_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._"

var auditRequiredColumns = []string{"id", "area", "title", "priority", "size"}

// auditScanner는 PROJECT_AUDIT.md를 한 줄씩 읽으며 누적하는 상태다.
type auditScanner struct {
	path         string
	lines        []string
	hasHeadings  bool
	openLevel    int
	header       map[string]int
	width        int
	inTable      bool
	validTable   bool
	explicitNone bool
	items        []contract.AuditItem
	warnings     []string
}

func CollectAuditItems(root string) ([]contract.AuditItem, []string) {
	path := filepath.Join(root, ".issueops", "PROJECT_AUDIT.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{"audit scan: " + err.Error()}
	}
	lines := strings.Split(string(b), "\n")
	s := &auditScanner{
		path:        path,
		lines:       lines,
		hasHeadings: hasAuditHeading(lines),
		items:       []contract.AuditItem{},
		warnings:    []string{},
	}
	for i := 0; i < len(lines); i++ {
		if s.scanLine(i) {
			i++ // 헤더 구분선 줄을 함께 소비한다.
		}
	}
	if !s.validTable && !s.explicitNone {
		s.warn(1, "no valid open audit table or explicit zero declaration")
	}
	if s.explicitNone && len(s.items) > 0 {
		s.warn(1, "explicit zero declaration contradicts open audit items")
	}
	return s.items, s.warnings
}

func hasAuditHeading(lines []string) bool {
	for _, line := range lines {
		if level, _ := auditHeading(strings.TrimSpace(line)); level > 0 {
			return true
		}
	}
	return false
}

func (s *auditScanner) warn(line int, reason string) {
	s.warnings = append(s.warnings, fmt.Sprintf("audit scan %s:%d: %s", s.path, line, reason))
}

// scanLine은 i번째 줄을 처리하고, 다음 줄(표 구분선)까지 소비했으면 true를 돌려준다.
func (s *auditScanner) scanLine(i int) bool {
	line := strings.TrimSpace(s.lines[i])
	if level, title := auditHeading(line); level > 0 {
		s.enterHeading(level, title)
		return false
	}
	if s.hasHeadings && s.openLevel == 0 {
		return false
	}
	if s.openLevel > 0 && line == auditNoneDeclaration {
		s.explicitNone = true
	}
	if !strings.HasPrefix(line, "|") {
		s.leaveTable(i, line)
		return false
	}
	parts := splitMarkdownRow(line)
	if !s.inTable {
		return s.startTable(i, parts)
	}
	if s.header == nil {
		return false
	}
	s.scanRow(i, parts)
	return false
}

func (s *auditScanner) enterHeading(level int, title string) {
	if s.openLevel > 0 && level <= s.openLevel {
		s.openLevel = 0
	}
	if title == "open" || strings.HasPrefix(title, "open ") {
		s.openLevel = level
	}
	s.inTable = false
}

// leaveTable은 파이프로 시작하지 않는 줄을 만났을 때 표를 닫고 누락된 선행 파이프를 경고한다.
func (s *auditScanner) leaveTable(i int, line string) {
	if s.inTable && s.header != nil && strings.Contains(line, "|") {
		s.warn(i+1, "audit row is missing its leading pipe")
	}
	s.inTable = false
}

// startTable은 헤더 줄을 검증하고, 구분선까지 소비했으면 true를 돌려준다.
func (s *auditScanner) startTable(i int, parts []string) bool {
	s.inTable = true
	s.width = len(parts)
	s.header = auditColumns(parts)
	if s.header == nil {
		s.warn(i+1, "invalid audit header: require unique ID, Area, Title, Priority, Size columns")
		return false
	}
	if i+1 >= len(s.lines) || !auditSeparator(strings.TrimSpace(s.lines[i+1]), s.width) {
		s.warn(i+1, "missing or invalid audit table separator")
		s.header = nil
		return false
	}
	s.validTable = true
	return true
}

func (s *auditScanner) scanRow(i int, parts []string) {
	if len(parts) != s.width {
		s.warn(i+1, "audit row width does not match header")
		return
	}
	if s.warnEmptyRequiredCells(i, parts) {
		return
	}
	priority := parts[s.header["priority"]]
	if priority != "P0" && priority != "P1" && priority != "P2" && priority != "P3" {
		s.warn(i+1, "invalid audit priority: require P0, P1, P2 or P3")
		return
	}
	if priority == "P3" {
		return
	}
	s.items = append(s.items, contract.AuditItem{
		ID:       parts[s.header["id"]],
		Area:     parts[s.header["area"]],
		Title:    parts[s.header["title"]],
		Priority: priority,
		Size:     parts[s.header["size"]],
	})
}

// warnEmptyRequiredCells는 비어 있는 필수 셀마다 경고하고, 하나라도 있으면 true를 돌려준다.
func (s *auditScanner) warnEmptyRequiredCells(i int, parts []string) bool {
	missing := false
	for _, name := range auditRequiredColumns {
		if parts[s.header[name]] == "" {
			s.warn(i+1, "empty required audit cell: "+name)
			missing = true
		}
	}
	return missing
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
