//go:build unix

package hostprobe

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"issueops/internal/port"
)

type homeTreeEntry struct {
	Path    string
	Mode    fs.FileMode
	Size    int64
	ModTime time.Time
	SHA256  string
}

func TestOmoRunnerKeepsVersionAndEpisodeWritesInsidePrivateHomes(t *testing.T) {
	originalHome := t.TempDir()
	originalAgent := filepath.Join(originalHome, ".omo", "agent")
	if err := os.MkdirAll(originalAgent, 0o700); err != nil {
		t.Fatal(err)
	}
	authPath := filepath.Join(originalAgent, "auth.json")
	if err := os.WriteFile(authPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixedTime := time.Unix(1_700_000_000, 0)
	if err := os.Chtimes(authPath, fixedTime, fixedTime); err != nil {
		t.Fatal(err)
	}
	before := snapshotHomeTree(t, originalHome)

	launcher, err := filepath.Abs(filepath.Join("testdata", "omo-home-writer.sh"))
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
	runner := NewOmoRunner(harness, omoTestLifecycleExtension(harness), Dependencies{
		Process: ExecRunner{},
		LookPath: func(name string) (string, error) {
			if name != "omo" {
				t.Fatalf("looked up %q", name)
			}
			return launcher, nil
		},
		TempDir: func(_, pattern string) (string, error) {
			if pattern != "issueops-conformance-omo-" || nextRoot >= len(roots) {
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

	request := omoProbeRequest()
	preflight := runner.Preflight(context.Background(), request)
	if !preflight.Ready || !preflight.Installed {
		t.Fatalf("preflight = %+v", preflight)
	}
	assertRemoved(t, roots[0])
	assertHomeTreeEqual(t, before, snapshotHomeTree(t, originalHome))

	result := runner.Run(context.Background(), request)
	if !result.Completed {
		t.Fatalf("episode = %+v", result)
	}
	assertRemoved(t, roots[1])
	after := snapshotHomeTree(t, originalHome)
	assertHomeTreeEqual(t, before, after)
	for _, forbidden := range []string{"launcher-state", "backup", "marker"} {
		for _, entry := range after {
			if strings.Contains(strings.ToLower(entry.Path), forbidden) {
				t.Fatalf("original HOME retained %q artifact: %s", forbidden, entry.Path)
			}
		}
	}
}

func TestResolveOmoAuthRejectsSymlinkToRegularAuthFile(t *testing.T) {
	home := t.TempDir()
	agentDir := filepath.Join(home, ".omo", "agent")
	if err := os.MkdirAll(agentDir, 0o700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(home, "real-auth.json")
	if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(agentDir, "auth.json")); err != nil {
		t.Fatal(err)
	}

	_, err := resolveOmoAuth(normalizeDependencies(Dependencies{Getenv: func(name string) string {
		if name == "HOME" {
			return home
		}
		return ""
	}}))
	if err == nil {
		t.Fatal("symlinked auth file was accepted")
	}
}

func snapshotHomeTree(t *testing.T, root string) []homeTreeEntry {
	t.Helper()
	entries := []homeTreeEntry{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		item := homeTreeEntry{Path: relative, Mode: info.Mode(), Size: info.Size(), ModTime: info.ModTime()}
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			item.SHA256 = fmt.Sprintf("%x", sha256.Sum256(body))
		}
		entries = append(entries, item)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

func assertHomeTreeEqual(t *testing.T, want, got []homeTreeEntry) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("original HOME changed\n got: %#v\nwant: %#v", got, want)
	}
}

func assertRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("private root %s was not removed: %v", path, err)
	}
}

var _ port.HostProbeRunner = OmoRunner{}
