package mcpcli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"issueops/cmd/issueops/mcpcli/argmap"
	authoritycontract "issueops/internal/contract/authority"
	authorityport "issueops/internal/port/authority"
)

// RecordRoots returns the workspace roots stored on a record: the source repo
// first, then any recorded worktree.
type RecordRoots func(ctx context.Context, kind RecordKind, id string) ([]string, error)

type requestAuthorityError struct{ code, reason string }

func (e *requestAuthorityError) Error() string { return e.code + ": " + e.reason }

func authorityRequired(reason string) error {
	return &requestAuthorityError{code: authoritycontract.CodeRequired, reason: reason}
}

func authorityInvalid(reason string) error {
	return &requestAuthorityError{code: authoritycontract.CodeInvalid, reason: reason}
}

// NewRequestScope resolves a workspace tool's scope only from the request and
// the stored record. It never consults the server cwd or environment.
func NewRequestScope(resolver authorityport.ScopeResolver, records RecordRoots) func(context.Context, string, map[string]any) (authoritycontract.Scope, error) {
	return func(ctx context.Context, tool string, args map[string]any) (authoritycontract.Scope, error) {
		spec := mcpToolAuthorities[tool]
		if spec.scope != toolScopeWorkspace {
			return authoritycontract.Scope{}, authorityInvalid("tool " + tool + " has no workspace scope")
		}
		cwd := strings.TrimSpace(argmap.String(args, argCWD))
		root, err := explicitRequestRoot(args, spec, cwd)
		if err != nil {
			return authoritycontract.Scope{}, err
		}
		if spec.record != recordNone {
			root, err = recordRequestRoot(ctx, records, spec.record, argmap.String(args, "id"), root, cwd)
			if err != nil {
				return authoritycontract.Scope{}, err
			}
		}
		if root == "" {
			return authoritycontract.Scope{}, authorityRequired("workspace_root is required for workspace tools")
		}
		if cwd != "" && !filepath.IsAbs(cwd) {
			cwd = filepath.Join(root, cwd)
		}
		scope, err := resolver.Resolve(ctx, root, cwd)
		if err != nil {
			return authoritycontract.Scope{}, authorityInvalid(fmt.Sprintf("request workspace scope: %v", err))
		}
		if err := confineRequestFiles(tool, args, scope); err != nil {
			return authoritycontract.Scope{}, err
		}
		return scope, nil
	}
}

// confinedFiles names the file inputs a tool reads itself. cwdRelative ones
// resolve against the request cwd; rootRelative ones against the scope root.
type confinedFiles struct{ cwdRelative, rootRelative []string }

var confinedFileArgs = map[string]confinedFiles{
	"api_doc_review":       {cwdRelative: []string{"diff_file", "prompt_file"}, rootRelative: []string{"files", "result_file"}},
	"api_doc_static_check": {rootRelative: []string{"files"}},
}

// confineRequestFiles rejects, before any file is read, a capability-scoped
// file input that resolves outside the verified scope root.
func confineRequestFiles(tool string, args map[string]any, scope authoritycontract.Scope) error {
	spec, ok := confinedFileArgs[tool]
	if !ok {
		return nil
	}
	root, err := os.OpenRoot(scope.WorkspaceRoot)
	if err != nil {
		return authorityInvalid(fmt.Sprintf("workspace root: %v", err))
	}
	defer root.Close()
	for _, group := range []struct {
		fields []string
		base   string
	}{{spec.cwdRelative, scope.CWD}, {spec.rootRelative, scope.WorkspaceRoot}} {
		for _, field := range group.fields {
			for _, value := range requestFileValues(args[field]) {
				if strings.TrimSpace(value) == "" {
					continue
				}
				if err := confineFile(root, scope.WorkspaceRoot, requestRelativePath(value, group.base)); err != nil {
					return authorityInvalid(field + " is outside the authorized workspace")
				}
			}
		}
	}
	return nil
}

func requestFileValues(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		var values []string
		for _, item := range typed {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return values
	default:
		return nil
	}
}

// confineFile maps path to a root-relative name and lets os.Root refuse ".."
// and symlink escapes. Only a missing target is tolerated; the later read fails.
func confineFile(root *os.Root, scopeRoot, path string) error {
	resolved, err := resolveExistingAncestor(filepath.Clean(path))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(scopeRoot, resolved)
	if err != nil || !filepath.IsLocal(relative) {
		return fs.ErrPermission
	}
	if _, err := root.Stat(relative); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// resolveExistingAncestor evaluates symlinks in the deepest existing ancestor
// so aliased spellings of the root (/tmp versus /private/tmp) compare equal.
func resolveExistingAncestor(path string) (string, error) {
	var tail []string
	for current := path; ; current = filepath.Dir(current) {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Join(append([]string{resolved}, tail...)...), nil
		}
		if !errors.Is(err, fs.ErrNotExist) || filepath.Dir(current) == current {
			return "", err
		}
		tail = append([]string{filepath.Base(current)}, tail...)
	}
}

func explicitRequestRoot(args map[string]any, spec toolAuthority, cwd string) (string, error) {
	var root string
	for _, field := range append([]string{argWorkspaceRoot}, spec.rootArgs...) {
		value := strings.TrimSpace(argmap.String(args, field))
		if value == "" {
			continue
		}
		if !filepath.IsAbs(value) {
			if !filepath.IsAbs(cwd) {
				return "", authorityInvalid(field + " must be absolute unless cwd is absolute")
			}
			value = filepath.Join(cwd, value)
		}
		canonical, err := canonicalRequestPath(value)
		if err != nil {
			return "", authorityInvalid(fmt.Sprintf("%s: %v", field, err))
		}
		if root != "" && canonical != root {
			return "", authorityInvalid(field + " conflicts with another workspace root input")
		}
		root = canonical
	}
	return root, nil
}

// recordRequestRoot pins the scope to a root the record itself names. An
// explicit root must be one of them; otherwise the deepest root holding the
// request cwd wins, then the source repo.
func recordRequestRoot(ctx context.Context, records RecordRoots, kind RecordKind, id, explicit, cwd string) (string, error) {
	if strings.TrimSpace(id) == "" {
		return explicit, nil
	}
	if records == nil {
		return "", authorityInvalid("record workspace lookup is not configured")
	}
	stored, err := records(ctx, kind, id)
	if err != nil {
		return "", err
	}
	var roots []string
	for _, candidate := range stored {
		if canonical, err := canonicalRequestPath(candidate); err == nil {
			roots = append(roots, canonical)
		}
	}
	if len(roots) == 0 {
		return "", authorityInvalid("record " + id + " names no existing workspace")
	}
	if explicit != "" {
		for _, root := range roots {
			if root == explicit {
				return explicit, nil
			}
		}
		return "", authorityInvalid("workspace root conflicts with record " + id)
	}
	if filepath.IsAbs(cwd) {
		canonicalCWD, err := canonicalRequestPath(cwd)
		if err != nil {
			return "", authorityInvalid(fmt.Sprintf("cwd: %v", err))
		}
		best := ""
		for _, root := range roots {
			if pathWithin(canonicalCWD, root) && len(root) > len(best) {
				best = root
			}
		}
		if best == "" {
			return "", authorityInvalid("cwd is outside the workspace of record " + id)
		}
		return best, nil
	}
	return roots[0], nil
}

func canonicalRequestPath(path string) (string, error) {
	return filepath.EvalSymlinks(filepath.Clean(path))
}

func pathWithin(path, root string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// scopedArguments rewrites the request onto its verified scope: root inputs
// become the scope root, cwd the scope cwd, and request-relative files absolute.
func scopedArguments(args map[string]any, spec toolAuthority, scope authoritycontract.Scope) map[string]any {
	scoped := make(map[string]any, len(args)+2)
	for key, value := range args {
		scoped[key] = value
	}
	delete(scoped, argAuthorityFile)
	scoped[argWorkspaceRoot] = scope.WorkspaceRoot
	scoped[argCWD] = scope.CWD
	for _, field := range spec.rootArgs {
		scoped[field] = scope.WorkspaceRoot
	}
	for _, field := range spec.pathArgs {
		switch value := scoped[field].(type) {
		case string:
			scoped[field] = requestRelativePath(value, scope.CWD)
		case []any:
			paths := make([]any, len(value))
			for index, item := range value {
				if text, ok := item.(string); ok {
					paths[index] = requestRelativePath(text, scope.CWD)
				} else {
					paths[index] = item
				}
			}
			scoped[field] = paths
		}
	}
	return scoped
}

func requestRelativePath(path, cwd string) string {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}
