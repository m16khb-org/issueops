//go:build unix

package hostprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"issueops/internal/adapter/hostprotocol"
)

func TestOpencodeRunnerNeverWritesUserHome(t *testing.T) {
	originalHome := t.TempDir()
	for _, path := range []string{
		filepath.Join(originalHome, ".config", "opencode", "opencode.json"),
		filepath.Join(originalHome, ".local", "share", "opencode", "opencode.db"),
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("user\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	before := snapshotHomeTree(t, originalHome)

	launcher, err := filepath.Abs(filepath.Join("testdata", "opencode-home-writer.sh"))
	if err != nil {
		t.Fatal(err)
	}
	harness, err := filepath.Abs(filepath.Join("testdata", "issueops"))
	if err != nil {
		t.Fatal(err)
	}
	tempParent := t.TempDir()
	roots := []string{filepath.Join(tempParent, "preflight"), filepath.Join(tempParent, "episode")}
	nextRoot := 0
	runner := NewOpencodeRunner(harness, opencodeTestPlugin(harness), Dependencies{
		Process: ExecRunner{},
		LookPath: func(name string) (string, error) {
			if name != "opencode" {
				t.Fatalf("looked up %q", name)
			}
			return launcher, nil
		},
		TempDir: func(_, pattern string) (string, error) {
			if pattern != "issueops-conformance-opencode-" || nextRoot >= len(roots) {
				return "", fmt.Errorf("unexpected temp request %q", pattern)
			}
			root := roots[nextRoot]
			nextRoot++
			return root, os.Mkdir(root, 0o700)
		},
		Environ: func() []string {
			return []string{"HOME=" + originalHome, "PATH=/usr/bin:/bin", "USER=fixture", "XDG_DATA_HOME=" + filepath.Join(originalHome, ".local", "share")}
		},
		HTTPGet: func(_ context.Context, address, _, _ string) (int, []byte, error) {
			return 200, []byte(`{"data":[{"name":"issueops_probe","status":{"status":"connected"}}]}`), nil
		},
	}, hostprotocol.OpencodeLifecyclePlugin)

	request := opencodeProbeRequest()
	if preflight := runner.Preflight(context.Background(), request); !preflight.Ready || preflight.Version != "opencode v2.0.32" {
		t.Fatalf("preflight = %+v", preflight)
	}
	assertRemoved(t, roots[0])
	assertHomeTreeEqual(t, before, snapshotHomeTree(t, originalHome))

	result := runner.Run(context.Background(), request)
	if !result.Completed || !result.SessionStartObserved {
		t.Fatalf("episode = %+v", result)
	}
	assertRemoved(t, roots[1])
	assertHomeTreeEqual(t, before, snapshotHomeTree(t, originalHome))
}
