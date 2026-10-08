//go:build unix

package hostprobe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOmpRunnerKeepsVersionAndEpisodeWritesInsidePrivateHomes(t *testing.T) {
	originalHome := t.TempDir()
	originalAgent := filepath.Join(originalHome, ".omp", "agent")
	if err := os.MkdirAll(originalAgent, 0o700); err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(originalAgent, "agent.db")
	writeOmpAuthStore(t, storePath)
	fixedTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(storePath, fixedTime, fixedTime); err != nil {
		t.Fatal(err)
	}
	before := snapshotHomeTree(t, originalHome)

	launcher, err := filepath.Abs(filepath.Join("testdata", "omp-home-writer.sh"))
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
	runner := newTestOmpRunner(harness, ompTestLifecycleExtension(harness), Dependencies{
		Process: ExecRunner{},
		LookPath: func(name string) (string, error) {
			if name != "omp" {
				t.Fatalf("looked up %q", name)
			}
			return launcher, nil
		},
		TempDir: func(_, pattern string) (string, error) {
			if pattern != "issueops-conformance-omp-" || nextRoot >= len(roots) {
				return "", fmt.Errorf("unexpected temp request %q", pattern)
			}
			root := roots[nextRoot]
			nextRoot++
			if err := os.Mkdir(root, 0o700); err != nil {
				return "", err
			}
			return root, nil
		},
		Getenv: func(name string) string {
			if name == "HOME" {
				return originalHome
			}
			return ""
		},
		Environ: func() []string {
			return []string{"HOME=" + originalHome, "PATH=/usr/bin:/bin", "USER=fixture"}
		},
	})

	request := ompProbeRequest()
	preflight := runner.Preflight(context.Background(), request)
	if !preflight.Ready || !preflight.Installed {
		t.Fatalf("preflight = %+v", preflight)
	}
	assertRemoved(t, roots[0])
	assertHomeTreeEqual(t, before, snapshotHomeTree(t, originalHome))

	result := runner.Run(context.Background(), request)
	if !result.Completed || result.ObservedModel != request.Model {
		t.Fatalf("episode = %+v", result)
	}
	assertRemoved(t, roots[1])
	after := snapshotHomeTree(t, originalHome)
	assertHomeTreeEqual(t, before, after)
	for _, forbidden := range []string{"launcher-state", "journal", "-wal", "-shm"} {
		for _, entry := range after {
			if strings.Contains(strings.ToLower(entry.Path), forbidden) {
				t.Fatalf("original HOME retained %q artifact: %s", forbidden, entry.Path)
			}
		}
	}
}
