// Package pioneerskill holds the catalog of canonical pioneer skill names and
// the pure check for which of them are absent. It performs no I/O.
package pioneerskill

var canonicalNames = []string{
	"web-research",
	"requirements-analysis",
	"architecture-review",
	"database-schema-architecture",
	"algorithm-optimization",
	"meeting-notes",
	"issueops-debugging",
	"prompt-engineering",
	"code-quality-metrics",
	"git-operations",
	"verified-execution",
	"implementation-planning",
}

// Names returns a copy of the canonical pioneer skill names in catalog order.
func Names() []string {
	return append([]string(nil), canonicalNames...)
}

// Missing returns the canonical pioneer skill names that do not appear in
// observed, in catalog order. It returns an empty non-nil slice when none are missing.
func Missing(observed []string) []string {
	present := make(map[string]bool, len(observed))
	for _, name := range observed {
		present[name] = true
	}
	missing := make([]string, 0)
	for _, name := range canonicalNames {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	return missing
}
