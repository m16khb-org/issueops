package upstream

import (
	"context"
	"path/filepath"
	"testing"

	"issueops/internal/testsupport"
)

func TestClaudePluginHostInstallPluginUsesSupportedArguments(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "claude")
	script := `#!/bin/sh
actual="$(printf '%s\n' "$@")"
expected='plugin
install
eli5@claude-community
--scope
user'
if [ "$actual" != "$expected" ]; then
	printf 'unexpected args:\n%s\n' "$actual" >&2
	exit 64
fi
`
	binary = testsupport.WriteFakeCommand(t, filepath.Dir(binary), "claude", script)

	host := ClaudePluginHost{Binary: binary}
	if err := host.InstallPlugin(context.Background(), "eli5@claude-community"); err != nil {
		t.Fatal(err)
	}
}
