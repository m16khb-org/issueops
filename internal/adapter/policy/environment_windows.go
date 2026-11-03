//go:build windows

package policy

import (
	"strings"

	policydomain "issueops/internal/domain/policy"
)

// commandEnvironment matches allowlist names case-insensitively, as Windows
// resolves environment names: an allowlisted PATH passes the Path entry.
func commandEnvironment(environ, allowlist []string) []string {
	names := make([]string, 0, len(allowlist))
	for _, wanted := range policydomain.CleanEnvAllowlist(allowlist) {
		names = append(names, wanted)
		for _, entry := range environ {
			if name, _, ok := strings.Cut(entry, "="); ok && name != wanted && strings.EqualFold(name, wanted) {
				names = append(names, name)
			}
		}
	}
	return policydomain.CommandEnvironment(environ, names)
}
