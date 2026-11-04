package projectdoc

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// PrefixedKnownProjectDocNames returns every required and optional project doc
// path. Optional docs render only when their condition holds (for example
// DESIGN.md only for repositories with a client surface), so iteration over
// this list must skip empty render results.
func PrefixedKnownProjectDocNames() []string {
	names := append(Names(), optionalProjectDocNames...)
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, filepath.ToSlash(filepath.Join(ProjectDocsDir, name)))
	}
	return out
}

// NormalizeRelPath converts a user-supplied document path into a slash-separated
// path under ProjectDocsDir. It accepts the known required and optional document
// names and family module documents, and returns an error for an empty path or
// any other target.
func NormalizeRelPath(relPath string) (string, error) {
	rel := filepath.ToSlash(strings.TrimSpace(relPath))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "" {
		return "", fmt.Errorf("rel_path is required")
	}
	if after, ok := strings.CutPrefix(rel, ProjectDocsDir+"/"); ok {
		rel = after
	}
	names := allowedProjectDocNames()
	allowed := map[string]bool{}
	for _, name := range names {
		allowed[name] = true
	}
	if !allowed[rel] {
		// Family module documents (adr/<record>.md, testing/overview.md, ...)
		// are valid revision targets in folder-first repositories.
		if _, ok := familyModulePath(rel); ok {
			return filepath.ToSlash(filepath.Join(ProjectDocsDir, rel)), nil
		}
		return "", fmt.Errorf("unsupported project doc %q: use one of %s", relPath, strings.Join(names, ", "))
	}
	return filepath.ToSlash(filepath.Join(ProjectDocsDir, rel)), nil
}

// NonEmptyStrings returns the trimmed items that are not blank, in order. The result is never nil.
func NonEmptyStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// AppendUnique appends v to items unless items already contains it.
func AppendUnique(items []string, v string) []string {
	if slices.Contains(items, v) {
		return items
	}
	return append(items, v)
}
