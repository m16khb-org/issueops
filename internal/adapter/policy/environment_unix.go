//go:build unix

package policy

import policydomain "issueops/internal/domain/policy"

func commandEnvironment(environ, allowlist []string) []string {
	return policydomain.CommandEnvironment(environ, allowlist)
}
