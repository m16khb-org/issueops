package preflight

import (
	"strings"

	"issueops/internal/adapter/outbound/gitraw"
)

func GitCmd(dir string, args ...string) (int, string, string) {
	code, stdout, stderr := GitCmdRaw(dir, args...)
	return code, strings.TrimSpace(stdout), strings.TrimSpace(stderr)
}

// GitCmdRaw preserves stdout byte-for-byte for machine-delimited Git output
// such as -z path lists. Human-facing callers should continue using GitCmd.
func GitCmdRaw(dir string, args ...string) (int, string, string) {
	return gitraw.Run(dir, args...)
}

func GitOut(dir string, args ...string) string {
	code, out, _ := GitCmd(dir, args...)
	if code != 0 {
		return ""
	}
	return strings.TrimSpace(out)
}
