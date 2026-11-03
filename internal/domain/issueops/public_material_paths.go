package issueops

import (
	"regexp"
	"slices"
	"strings"
	"sync"
)

// URLs are opaque at this boundary, including their path, query, and fragment.
// A scheme has two or more characters, so a drive path such as C:\work is not a URL.
var publicMaterialURL = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile("(^|[\\s`'\"(<\\[=:])([A-Za-z][A-Za-z0-9+.-]+:[^\\s<>\"`]+|//[^\\s<>\"`]+)")
})

type publicMaterialRoot struct{ root, label string }

// publicMaterialRoots lists the absolute roots to relabel, longest first.
func publicMaterialRoots(sourceRoot, worktreeRoot string) []publicMaterialRoot {
	var roots []publicMaterialRoot
	for _, candidate := range []publicMaterialRoot{{sourceRoot, "$SOURCE_ROOT"}, {worktreeRoot, "$WORKTREE"}} {
		if drivePublicMaterialRoot(candidate.root) {
			roots = append(roots, drivePublicMaterialRoots(candidate)...)
			continue
		}
		candidate.root = strings.TrimRight(candidate.root, "/")
		if !strings.HasPrefix(candidate.root, "/") || strings.HasPrefix(candidate.root, "//") {
			continue
		}
		roots = append(roots, candidate)
		parts := strings.SplitN(candidate.root, "/", 4)
		if len(parts) >= 3 && (parts[1] == "Users" || parts[1] == "home") && parts[2] != "" && parts[2] != "." && parts[2] != ".." {
			roots = append(roots, publicMaterialRoot{"/" + parts[1] + "/" + parts[2], "$HOME"})
		}
	}
	slices.SortStableFunc(roots, func(a, b publicMaterialRoot) int { return len(b.root) - len(a.root) })
	return roots
}

// drivePublicMaterialRoot reports a Windows drive root such as C:\work or C:/work.
func drivePublicMaterialRoot(root string) bool {
	return len(root) >= 3 && root[1] == ':' && (root[2] == '\\' || root[2] == '/') &&
		(root[0] >= 'A' && root[0] <= 'Z' || root[0] >= 'a' && root[0] <= 'z')
}

// drivePublicMaterialRoots relabels a Windows root in both separator forms,
// since Windows tools print backslashes and Git prints forward slashes, and
// relabels C:\Users\<name> as $HOME.
func drivePublicMaterialRoots(candidate publicMaterialRoot) []publicMaterialRoot {
	native := strings.TrimRight(strings.ReplaceAll(candidate.root, "/", `\`), `\`)
	if len(native) < 3 {
		return nil
	}
	roots := []publicMaterialRoot{{native, candidate.label}, {strings.ReplaceAll(native, `\`, "/"), candidate.label}}
	parts := strings.SplitN(native, `\`, 4)
	if len(parts) >= 3 && strings.EqualFold(parts[1], "Users") && parts[2] != "" && parts[2] != "." && parts[2] != ".." {
		home := parts[0] + `\` + parts[1] + `\` + parts[2]
		roots = append(roots, publicMaterialRoot{home, "$HOME"}, publicMaterialRoot{strings.ReplaceAll(home, `\`, "/"), "$HOME"})
	}
	return roots
}

// matchPublicMaterialRoot reports the root label and end offset when a root starts at text[i].
func matchPublicMaterialRoot(text string, i int, roots []publicMaterialRoot) (string, int, bool) {
	if i != 0 && !strings.ContainsRune(" \t\r\n`'\"(<[=:", rune(text[i-1])) {
		return "", 0, false
	}
	for _, root := range roots {
		end := i + len(root.root)
		if strings.HasPrefix(text[i:], root.root) && (end == len(text) || strings.ContainsRune("/\\ \t\r\n`'\")>],;:", rune(text[end]))) {
			return root.label, end, true
		}
	}
	return "", 0, false
}

// NormalizePublicMaterialPaths changes only public copies. Roots come from the
// record, so the rule neither observes the filesystem nor guesses other homes.
func NormalizePublicMaterialPaths(content []byte, sourceRoot, worktreeRoot string) []byte {
	roots := publicMaterialRoots(sourceRoot, worktreeRoot)
	text := string(content)
	var out strings.Builder
	out.Grow(len(text))
	urls := publicMaterialURL().FindAllStringIndex(text, -1)
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
		if label, end, ok := matchPublicMaterialRoot(text, i, roots); ok {
			out.WriteString(label)
			i = end
			continue
		}
		out.WriteByte(text[i])
		i++
	}
	return []byte(out.String())
}
