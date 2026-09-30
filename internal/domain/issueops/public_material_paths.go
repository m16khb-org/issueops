package issueops

import (
	"regexp"
	"slices"
	"strings"
)

// URLs are opaque at this boundary, including their path, query, and fragment.
var publicMaterialURL = regexp.MustCompile("(^|[\\s`'\"(<\\[=:])([A-Za-z][A-Za-z0-9+.-]*:[^\\s<>\"`]+|//[^\\s<>\"`]+)")

// NormalizePublicMaterialPaths changes only public copies. Roots come from the
// record, so the rule neither observes the filesystem nor guesses other homes.
func NormalizePublicMaterialPaths(content []byte, sourceRoot, worktreeRoot string) []byte {
	type replacement struct{ root, label string }
	var roots []replacement
	for _, candidate := range []replacement{{sourceRoot, "$SOURCE_ROOT"}, {worktreeRoot, "$WORKTREE"}} {
		candidate.root = strings.TrimRight(candidate.root, "/")
		if !strings.HasPrefix(candidate.root, "/") || strings.HasPrefix(candidate.root, "//") {
			continue
		}
		roots = append(roots, candidate)
		parts := strings.SplitN(candidate.root, "/", 4)
		if len(parts) >= 3 && (parts[1] == "Users" || parts[1] == "home") && parts[2] != "" && parts[2] != "." && parts[2] != ".." {
			roots = append(roots, replacement{"/" + parts[1] + "/" + parts[2], "$HOME"})
		}
	}
	slices.SortStableFunc(roots, func(a, b replacement) int { return len(b.root) - len(a.root) })
	text := string(content)
	var out strings.Builder
	out.Grow(len(text))
	urls := publicMaterialURL.FindAllStringIndex(text, -1)
	urlIndex := 0
	for i := 0; i < len(text); {
		for urlIndex < len(urls) && urls[urlIndex][0] < i {
			urlIndex++
		}
		if urlIndex < len(urls) && i == urls[urlIndex][0] {
			out.WriteString(text[i:urls[urlIndex][1]])
			i = urls[urlIndex][1]
			urlIndex++
			continue
		}
		matched := false
		if i == 0 || strings.ContainsRune(" \t\r\n`'\"(<[=:", rune(text[i-1])) {
			for _, root := range roots {
				end := i + len(root.root)
				if strings.HasPrefix(text[i:], root.root) && (end == len(text) || strings.ContainsRune("/ \t\r\n`'\")>],;:", rune(text[end]))) {
					out.WriteString(root.label)
					i = end
					matched = true
					break
				}
			}
		}
		if !matched {
			out.WriteByte(text[i])
			i++
		}
	}
	return []byte(out.String())
}
