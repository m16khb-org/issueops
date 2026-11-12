package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 호출자 context가 있는 경로에서 시작하는 자식 프로세스는 그 context를 받는다
// (exec.CommandContext). 호출자 context가 없는 경로는 CLI가 root context를 갖기 전까지
// 아래 allowlist에 사유와 함께 묶어 두는 예외다(.issueops/adr/2026-11-12-child-processes-take-the-caller-context-where-one-exists.md).
// 이 ratchet은 production 코드의 exec.Command를 선언 단위로 세고, allowlist에 없는 호출,
// 개수가 어긋난 항목(많든 적든), 남은 항목이 생기면 실패한다. allowlist가 없는 금지 형태도
// 함께 막는다: os/exec dot import, exec.Cmd 직접 생성, os.StartProcess와 syscall의 프로세스
// 시작 함수, exec.Cmd 값 선언(변수와 struct field), 첫 인자가 context.Background()나 context.TODO()
// 호출식인 exec.CommandContext.
//
// 알려진 한계: 판정은 구문만 본다. 변수를 거친 root context(ctx := context.Background();
// exec.CommandContext(ctx, ...))는 잡지 못한다. 이 형태는 리뷰에서 확인한다.

type execCommandSite struct {
	file     string
	function string
}

// execCommandClass는 allowlist 항목이 아직 exec.Command를 쓰는 이유의 분류다.
type execCommandClass string

const (
	// 프로세스 수명을 스스로 관리한다(process group kill, 호출보다 오래 사는 서버, 자체 deadline).
	execClassOwnLifecycle execCommandClass = "own-lifecycle"
	// 설치·업데이트 단계가 실행하는 installer라 중간 취소가 설치를 반쯤 남긴다.
	execClassInstaller execCommandClass = "installer"
	// 호출 경로 어디에도 context가 없다. CLI root context가 생긴 뒤에 바꾼다.
	execClassNoCallerContext execCommandClass = "no-caller-context"
	// 실패를 정상 값으로 읽는다. 취소가 조용한 오판정이 된다.
	execClassFailOpenOnCancel execCommandClass = "fail-open-on-cancel"
	// context를 넘기려면 넓은 시그니처 변경이 필요하다.
	execClassWideSignatureChange execCommandClass = "wide-signature-change"
)

type execCommandAllowance struct {
	count int
	class execCommandClass
	note  string
}

const (
	execCommandAllowlistEntries = 17
	execCommandAllowlistCalls   = 17
)

var execCommandAllowlist = map[execCommandSite]execCommandAllowance{
	{"internal/adapter/augmentation/repository.go", "Repository.HasImplementationDelta"}:   {1, execClassNoCallerContext, "self-augment planner port has no ctx"},
	{"internal/adapter/docs/docs.go", "Observer.TrackedPaths"}:                             {1, execClassNoCallerContext, "docs observer port has no ctx"},
	{"internal/adapter/guard/helpers.go", "gitCmd"}:                                        {1, execClassFailOpenOnCancel, "guard reads a git failure as an answer"},
	{"internal/adapter/hostprobe/opencode.go", "startExecServer"}:                          {1, execClassOwnLifecycle, "probe server outlives the call"},
	{"internal/adapter/hostprobe/runner.go", "execRunner.Run"}:                             {1, execClassOwnLifecycle, "kills the process group when ctx ends"},
	{"internal/adapter/issueops/material_files.go", "MaterialFiles.Ignored"}:               {1, execClassFailOpenOnCancel, "a git failure reads as not ignored"},
	{"internal/adapter/outbound/gitexclude/gitexclude.go", "EnsureWorkspaceState"}:         {1, execClassFailOpenOnCancel, "best effort; callers discard the error; a cancel would be fail-open"},
	{"internal/adapter/outbound/gitraw/gitraw.go", "Run"}:                                  {1, execClassWideSignatureChange, "21 function types in 14 files; a kill can interrupt a writing git command"},
	{"internal/adapter/outbound/issueopsinventory/runtime.go", "CleanPath.Normalize"}:      {1, execClassFailOpenOnCancel, "a git failure keeps the input path"},
	{"internal/adapter/outbound/quality/coverage.go", "gitCoverageFingerprintInput"}:       {1, execClassNoCallerContext, "RunGoTestCoverage builds its ctx from Background; a failure skips the cache"},
	{"internal/adapter/policy/policy_run.go", "CommandExecutor.Execute"}:                   {1, execClassOwnLifecycle, "own process-group supervision with its own deadline; passing the caller ctx is a held follow-up"},
	{"internal/adapter/processinspect/inspect_ps.go", "Inspector.processFieldWithCLocale"}: {1, execClassNoCallerContext, "update CLI and the MCP-service supervisor (mcpServiceProcessInspector); the port is func(int) with no ctx"},
	{"internal/adapter/update/runtime.go", "ProcessTable.Read"}:                            {1, execClassNoCallerContext, "reached from the update CLI"},
	{"internal/adapter/update/runtime_unix.go", "Runtime.Install"}:                         {1, execClassInstaller, "install script runs to completion"},
	{"internal/adapter/update/runtime_windows.go", "nativeInstallRun.exec"}:                {1, execClassInstaller, "installer runs to completion"},
	{"internal/adapter/update/runtime_windows.go", "stableNativeRoot"}:                     {1, execClassFailOpenOnCancel, "a git failure keeps the input root"},
	{"internal/adapter/verifywork/git.go", "GitStatus"}:                                    {1, execClassNoCallerContext, "verify-work service port has no ctx"},
}

func TestProductionExecCommandSitesMatchAllowlist(t *testing.T) {
	assertExecCommandAllowlistShape(t)
	repoRoot := findRepoRoot(t)
	found := map[execCommandSite]int{}
	var violations []string
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(repoRoot, top), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(repoRoot, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if entry.IsDir() {
				if entry.Name() == "testdata" || relative == "internal/testsupport" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(relative, ".go") || strings.HasSuffix(relative, "_test.go") {
				return nil
			}
			// build tag는 보지 않는다. parser는 //go:build 줄을 해석하지 않으므로 모든 GOOS의 파일을 센다.
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			scan := scanExecUse(fset, file)
			for _, use := range scan.commands {
				found[execCommandSite{file: relative, function: use.function}]++
			}
			for _, use := range scan.violations {
				violations = append(violations, fmt.Sprintf("%s:%d %s in %s", relative, use.line, use.what, use.function))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(violations)
	if len(violations) != 0 {
		t.Errorf("production code starts a child process in a forbidden form:\n%s", strings.Join(violations, "\n"))
	}
	var drift []string
	for site, count := range found {
		allowed, ok := execCommandAllowlist[site]
		switch {
		case !ok:
			drift = append(drift, fmt.Sprintf("new exec.Command in %s %s (%d): pass the caller ctx to exec.CommandContext, or add an allowlist entry with a class", site.file, site.function, count))
		case allowed.count != count:
			drift = append(drift, fmt.Sprintf("%s %s has %d exec.Command, allowlist says %d", site.file, site.function, count, allowed.count))
		}
	}
	for site := range execCommandAllowlist {
		if _, ok := found[site]; !ok {
			drift = append(drift, fmt.Sprintf("stale allowlist entry %s %s: remove it", site.file, site.function))
		}
	}
	sort.Strings(drift)
	if len(drift) != 0 {
		t.Errorf("exec.Command sites drifted from the allowlist:\n%s", strings.Join(drift, "\n"))
	}
}

// allowlist는 줄어들기만 해야 하므로 항목 수와 호출 수를 상수로 고정하고, 항목마다 정해진
// 분류와 비어 있지 않은 설명을 요구한다.
func assertExecCommandAllowlistShape(t *testing.T) {
	t.Helper()
	classes := []execCommandClass{execClassOwnLifecycle, execClassInstaller, execClassNoCallerContext, execClassFailOpenOnCancel, execClassWideSignatureChange}
	total := 0
	for site, allowed := range execCommandAllowlist {
		total += allowed.count
		if allowed.count < 1 {
			t.Errorf("allowlist entry %s %s has count %d", site.file, site.function, allowed.count)
		}
		if !slices.Contains(classes, allowed.class) {
			t.Errorf("allowlist entry %s %s has unknown class %q", site.file, site.function, allowed.class)
		}
		if strings.TrimSpace(allowed.note) == "" {
			t.Errorf("allowlist entry %s %s has no reason", site.file, site.function)
		}
	}
	if len(execCommandAllowlist) != execCommandAllowlistEntries {
		t.Errorf("allowlist has %d entries, want %d", len(execCommandAllowlist), execCommandAllowlistEntries)
	}
	if total != execCommandAllowlistCalls {
		t.Errorf("allowlist counts sum to %d, want %d", total, execCommandAllowlistCalls)
	}
}

// fixture는 디스크에 두지 않는다. internal 아래 .go 파일은 이 ratchet과 DDD source inventory가
// production 코드로 읽는다. 문자열 source를 production walk와 같은 parse와 scan에 넣는다.
func TestExecCommandScannerFixtures(t *testing.T) {
	for _, tc := range []struct {
		name           string
		source         string
		wantCommands   []string
		wantViolations []string
	}{
		{
			name:         "aliased import in a windows-only file",
			source:       "//go:build windows\n\npackage fixture\n\nimport x \"os/exec\"\n\nfunc start() { _ = x.Command(\"tool\") }\n",
			wantCommands: []string{"start"},
		},
		{
			name:   "comment only",
			source: "package fixture\n\nimport \"os/exec\"\n\n// start used to call exec.Command(\"tool\").\nfunc start() *exec.Cmd { return nil }\n",
		},
		{
			name:           "CommandContext with context.Background",
			source:         "package fixture\n\nimport (\n\t\"context\"\n\t\"os/exec\"\n)\n\nfunc start() { _ = exec.CommandContext(context.Background(), \"tool\") }\n",
			wantViolations: []string{"exec.CommandContext(context.Background())"},
		},
		{
			name:           "CommandContext with context.TODO",
			source:         "package fixture\n\nimport (\n\tctxpkg \"context\"\n\t\"os/exec\"\n)\n\ntype runner struct{}\n\nfunc (*runner) start() { _ = exec.CommandContext(ctxpkg.TODO(), \"tool\") }\n",
			wantViolations: []string{"exec.CommandContext(context.TODO())"},
		},
		{
			name:           "dot import",
			source:         "package fixture\n\nimport . \"os/exec\"\n\nfunc start() { _ = Command(\"tool\") }\n",
			wantViolations: []string{"dot import of os/exec"},
		},
		{
			name:           "Cmd literal",
			source:         "package fixture\n\nimport \"os/exec\"\n\nfunc start() { _ = &exec.Cmd{Path: \"tool\"} }\n",
			wantViolations: []string{"exec.Cmd literal"},
		},
		{
			name:         "method value",
			source:       "package fixture\n\nimport \"os/exec\"\n\nvar newCommand = exec.Command\n\nfunc start() { f := exec.Command; _ = f }\n",
			wantCommands: []string{"start", "var newCommand"},
		},
		{
			name:         "closure counts toward its method",
			source:       "package fixture\n\nimport \"os/exec\"\n\ntype runner[T any] struct{}\n\nfunc (r *runner[T]) start() { go func() { _ = exec.Command(\"tool\") }() }\n",
			wantCommands: []string{"runner.start"},
		},
		{
			name: "Cmd value variable and field",
			source: `package fixture

import "os/exec"

type runner struct{ cmd exec.Cmd }

func start() {
	var c exec.Cmd
	c.Path = "tool"
	_ = c.Start()
}
`,
			wantViolations: []string{"exec.Cmd value declaration", "exec.Cmd value field"},
		},
		{
			name: "Cmd pointer holding a Command result",
			source: `package fixture

import "os/exec"

type runner struct{ cmd *exec.Cmd }

func start() {
	var c *exec.Cmd = exec.Command("tool")
	_ = runner{cmd: c}
}
`,
			wantCommands: []string{"start"},
		},
		{
			name: "os/exec imported under two names",
			source: `package fixture

import (
	"os/exec"
	x "os/exec"
)

func start() {
	_ = exec.Command("tool")
	_ = x.Command("tool")
}
`,
			wantCommands: []string{"start", "start"},
		},
		{
			name:           "process start outside os/exec",
			source:         "package fixture\n\nimport (\n\t\"os\"\n\t\"syscall\"\n)\n\nfunc start() {\n\t_, _ = os.StartProcess(\"tool\", nil, nil)\n\t_, _ = syscall.ForkExec(\"tool\", nil, nil)\n\t_ = syscall.Exec(\"tool\", nil, nil)\n}\n",
			wantViolations: []string{"os.StartProcess", "syscall.Exec", "syscall.ForkExec"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "fixture.go", tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			scan := scanExecUse(fset, file)
			var commands, violations []string
			for _, use := range scan.commands {
				commands = append(commands, use.function)
			}
			for _, use := range scan.violations {
				violations = append(violations, use.what)
			}
			sort.Strings(commands)
			sort.Strings(violations)
			if !slices.Equal(commands, tc.wantCommands) {
				t.Errorf("exec.Command uses = %q, want %q", commands, tc.wantCommands)
			}
			if !slices.Equal(violations, tc.wantViolations) {
				t.Errorf("violations = %q, want %q", violations, tc.wantViolations)
			}
		})
	}
}

type execUse struct {
	function string
	line     int
	what     string
}

type execScan struct {
	commands   []execUse
	violations []execUse
}

// scanExecUse는 파일 하나에서 exec.Command selector(호출과 method value)와 금지 형태를 찾는다.
// closure는 자신을 둘러싼 선언(Recv.Method 또는 함수 이름)에 속한다.
func scanExecUse(fset *token.FileSet, file *ast.File) execScan {
	var scan execScan
	execNames, execDot := importedNames(file, "os/exec")
	osNames, _ := importedNames(file, "os")
	syscallNames, _ := importedNames(file, "syscall")
	contextNames, _ := importedNames(file, "context")
	if execDot {
		scan.violations = append(scan.violations, execUse{function: "imports", line: fset.Position(file.Package).Line, what: "dot import of os/exec"})
	}
	for _, decl := range file.Decls {
		owner := declarationOwner(decl)
		ast.Inspect(decl, func(node ast.Node) bool {
			use := func(what string) execUse {
				return execUse{function: owner, line: fset.Position(node.Pos()).Line, what: what}
			}
			switch node := node.(type) {
			case *ast.SelectorExpr:
				switch {
				case isPackageSelector(node, execNames, "Command"):
					scan.commands = append(scan.commands, use("exec.Command"))
				case isPackageSelector(node, osNames, "StartProcess"):
					scan.violations = append(scan.violations, use("os.StartProcess"))
				case isPackageSelector(node, syscallNames, "StartProcess"), isPackageSelector(node, syscallNames, "ForkExec"), isPackageSelector(node, syscallNames, "Exec"):
					scan.violations = append(scan.violations, use("syscall."+node.Sel.Name))
				}
			case *ast.ValueSpec:
				if isExecCmdValue(node.Type, execNames) {
					scan.violations = append(scan.violations, use("exec.Cmd value declaration"))
				}
			case *ast.StructType:
				for _, field := range node.Fields.List {
					if isExecCmdValue(field.Type, execNames) {
						scan.violations = append(scan.violations, execUse{function: owner, line: fset.Position(field.Pos()).Line, what: "exec.Cmd value field"})
					}
				}
			case *ast.CompositeLit:
				if selector, ok := node.Type.(*ast.SelectorExpr); ok && isPackageSelector(selector, execNames, "Cmd") {
					scan.violations = append(scan.violations, use("exec.Cmd literal"))
				}
			case *ast.CallExpr:
				if selector, ok := node.Fun.(*ast.SelectorExpr); ok && isPackageSelector(selector, execNames, "CommandContext") && len(node.Args) > 0 {
					if root := rootContextCall(node.Args[0], contextNames); root != "" {
						scan.violations = append(scan.violations, use("exec.CommandContext(context."+root+"())"))
					}
				}
				if ident, ok := node.Fun.(*ast.Ident); ok && ident.Name == "new" && len(node.Args) == 1 {
					if selector, ok := node.Args[0].(*ast.SelectorExpr); ok && isPackageSelector(selector, execNames, "Cmd") {
						scan.violations = append(scan.violations, use("new(exec.Cmd)"))
					}
				}
			}
			return true
		})
	}
	return scan
}

// importedNames는 path를 import한 파일 안의 모든 바인딩 이름과 dot import 여부를 돌려준다.
// 한 파일이 같은 패키지를 두 이름으로 import해도 둘 다 센다. blank import는 이름이 없다.
func importedNames(file *ast.File, path string) ([]string, bool) {
	var names []string
	dot := false
	for _, imported := range file.Imports {
		value, err := strconv.Unquote(imported.Path.Value)
		if err != nil || value != path {
			continue
		}
		switch {
		case imported.Name == nil:
			names = append(names, path[strings.LastIndex(path, "/")+1:])
		case imported.Name.Name == ".":
			dot = true
		case imported.Name.Name != "_":
			names = append(names, imported.Name.Name)
		}
	}
	return names, dot
}

// isExecCmdValue는 선언 타입이 포인터가 아닌 exec.Cmd인지 본다. *exec.Cmd는 exec.Command*의
// 결과를 담는 보통 형태라 통과한다.
func isExecCmdValue(expr ast.Expr, execNames []string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	return ok && isPackageSelector(selector, execNames, "Cmd")
}

func isPackageSelector(selector *ast.SelectorExpr, packageNames []string, name string) bool {
	if selector.Sel.Name != name {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && slices.Contains(packageNames, identifier.Name)
}

// rootContextCall은 expr가 context.Background()나 context.TODO() 호출이면 그 함수 이름을 돌려준다.
func rootContextCall(expr ast.Expr, contextNames []string) string {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return ""
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	for _, name := range []string{"Background", "TODO"} {
		if isPackageSelector(selector, contextNames, name) {
			return name
		}
	}
	return ""
}

// declarationOwner는 최상위 선언의 이름이다. method는 Recv.Method, 함수는 이름,
// package 수준 var/const는 "var name" 꼴이다.
func declarationOwner(decl ast.Decl) string {
	switch decl := decl.(type) {
	case *ast.FuncDecl:
		if decl.Recv == nil || len(decl.Recv.List) == 0 {
			return decl.Name.Name
		}
		return receiverTypeName(decl.Recv.List[0].Type) + "." + decl.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, spec := range decl.Specs {
			switch spec := spec.(type) {
			case *ast.ValueSpec:
				for _, name := range spec.Names {
					names = append(names, name.Name)
				}
			case *ast.TypeSpec:
				names = append(names, spec.Name.Name)
			}
		}
		return decl.Tok.String() + " " + strings.Join(names, ",")
	}
	return "?"
}

func receiverTypeName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.StarExpr:
		return receiverTypeName(expr.X)
	case *ast.IndexExpr:
		return receiverTypeName(expr.X)
	case *ast.IndexListExpr:
		return receiverTypeName(expr.X)
	case *ast.Ident:
		return expr.Name
	}
	return "?"
}
