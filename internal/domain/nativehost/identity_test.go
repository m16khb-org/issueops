package nativehost

import "testing"

func TestExecutableMatchesHost(t *testing.T) {
	for _, tc := range []struct {
		host, executable string
		want             bool
	}{
		{"codex", "/usr/local/bin/codex", true},
		{" Codex ", " /opt/tools/CODEX.EXE ", true},
		{"codex", "/usr/local/bin/codex-wrapper", false},
		{"claude", "claude", true},
		{"claude", "/Users/u/.local/share/claude/versions/2.1.0", true},
		{"claude", "/usr/bin/node", false},
		{"omo", "/opt/omo/bin/omo", true},
		{"omo", "/opt/omo/bin/codex", false},
		{"omp", "/Users/u/.bun/bin/omp", true},
		{"omp", "/Users/u/.bun/bin/omo", false},
		{"gemini", "/usr/bin/gemini", false},
		{"", "codex", false},
	} {
		if got := ExecutableMatchesHost(tc.host, tc.executable); got != tc.want {
			t.Errorf("ExecutableMatchesHost(%q, %q) = %v, want %v", tc.host, tc.executable, got, tc.want)
		}
	}
}

func TestExecutableMatchesHostOpencode(t *testing.T) {
	for _, tc := range []struct {
		executable string
		want       bool
	}{
		{"/opt/homebrew/Cellar/opencode-v2/2.0.32/bin/opencode", true},
		{" /opt/tools/OPENCODE.EXE ", true},
		{"opencode", true},
		{"/opt/homebrew/bin/opencode-wrapper", false},
		{"/Users/u/.bun/bin/omp", false},
	} {
		if got := ExecutableMatchesHost("opencode", tc.executable); got != tc.want {
			t.Errorf("ExecutableMatchesHost(opencode, %q) = %v, want %v", tc.executable, got, tc.want)
		}
	}
}
