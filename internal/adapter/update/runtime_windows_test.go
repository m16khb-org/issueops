//go:build windows

package update

import (
	"bytes"
	"strings"
	"testing"
)

// On Windows Install runs the Go counterpart of scripts/install-native.sh, so
// these cases exercise its argument handling and missing-binary reporting
// without building anything.
func TestRuntimeInstallWithoutBinary(t *testing.T) {
	t.Setenv("ISSUEOPS_SKIP_BUILD", "")
	t.Run("dry run reports the missing binary and succeeds", func(t *testing.T) {
		root := t.TempDir()
		var diag bytes.Buffer
		if err := (Runtime{Diagnostics: &diag}).Install(root, []string{"--dry-run"}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(diag.String(), "dry-run: binary missing; skipping install plan because ") {
			t.Fatalf("diagnostics = %q", diag.String())
		}
	})
	t.Run("nil diagnostics are discarded", func(t *testing.T) {
		if err := (Runtime{}).Install(t.TempDir(), []string{"--dry-run"}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("skip build without a binary fails", func(t *testing.T) {
		err := (Runtime{}).Install(t.TempDir(), []string{"--skip-build"})
		if err == nil || !strings.Contains(err.Error(), "issueops binary missing after build") {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("space separated transport is refused", func(t *testing.T) {
		err := (Runtime{}).Install(t.TempDir(), []string{"--mcp-transport", "http"})
		if err == nil || !strings.Contains(err.Error(), "use --mcp-transport=http or --mcp-transport=stdio") {
			t.Fatalf("got %v", err)
		}
	})
}
