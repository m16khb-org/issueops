//go:build aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris

package authority

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	domain "issueops/internal/domain/authority"
)

func credentialFixture(t *testing.T) (CredentialFiles, string, string) {
	t.Helper()
	state := t.TempDir()
	key := strings.Repeat("a", 64)
	token := domain.ComposeToken(key, "c2VjcmV0LXZhbHVl")
	return CredentialFiles{StateDir: state}, key, token
}

func TestCredentialFileIsImmutableOwnerOnlyAndRoundTrips(t *testing.T) {
	files, key, token := credentialFixture(t)
	path, err := files.Write(context.Background(), key, token)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(files.StateDir, "mcp-http", "grants", key, domain.TokenDigest(token)) {
		t.Fatalf("path=%s", path)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode().Perm() != 0o600 || !info.Mode().IsRegular() {
		t.Fatalf("credential mode=%v err=%v", info.Mode(), err)
	}
	for _, dir := range []string{filepath.Join(files.StateDir, "mcp-http"), filepath.Join(files.StateDir, "mcp-http", "grants"), filepath.Dir(path)} {
		dirInfo, err := os.Lstat(dir)
		if err != nil || dirInfo.Mode().Perm() != 0o700 {
			t.Fatalf("%s mode=%v err=%v", dir, dirInfo.Mode(), err)
		}
	}
	gotKey, gotToken, err := files.Read(context.Background(), path)
	if err != nil || gotKey != key || gotToken != token {
		t.Fatalf("read key=%s token-match=%v err=%v", gotKey, gotToken == token, err)
	}
	if _, err := files.Write(context.Background(), key, token); err == nil {
		t.Fatal("existing credential file was rewritten")
	}
	if _, err := files.Write(context.Background(), strings.Repeat("b", 64), token); err == nil {
		t.Fatal("credential written under a foreign key")
	}
}

func TestCredentialReadRejectsUnmanagedPaths(t *testing.T) {
	files, key, token := credentialFixture(t)
	path, err := files.Write(context.Background(), key, token)
	if err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(outside, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]string{
		"relative":        strings.TrimPrefix(path, "/"),
		"outside":         outside,
		"traversal":       filepath.Dir(path) + "/../" + key + "/" + filepath.Base(path),
		"grants dir":      filepath.Dir(filepath.Dir(path)),
		"extra component": filepath.Join(path, "x"),
		"non-hex digest":  filepath.Join(filepath.Dir(path), strings.Repeat("z", 64)),
		"missing digest":  filepath.Join(filepath.Dir(path), strings.Repeat("c", 64)),
	} {
		if _, _, err := files.Read(context.Background(), candidate); err == nil {
			t.Fatalf("%s path accepted", name)
		}
	}
}

func TestCredentialReadRejectsSymlinksModesAndForeignContent(t *testing.T) {
	cases := map[string]func(t *testing.T, files CredentialFiles, path string){
		"group readable file": func(t *testing.T, _ CredentialFiles, path string) { chmod(t, path, 0o640) },
		"world writable dir":  func(t *testing.T, _ CredentialFiles, path string) { chmod(t, filepath.Dir(path), 0o777) },
		"group grants dir":    func(t *testing.T, _ CredentialFiles, path string) { chmod(t, filepath.Dir(filepath.Dir(path)), 0o750) },
		"symlinked file": func(t *testing.T, _ CredentialFiles, path string) {
			target := filepath.Join(t.TempDir(), "target")
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(target, data, 0o600); err != nil {
				t.Fatal(err)
			}
			replaceWithSymlink(t, target, path)
		},
		"symlinked key dir": func(t *testing.T, _ CredentialFiles, path string) {
			target := t.TempDir()
			chmod(t, target, 0o700)
			data, _ := os.ReadFile(path)
			if err := os.WriteFile(filepath.Join(target, filepath.Base(path)), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Dir(path)); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Dir(path)); err != nil {
				t.Fatal(err)
			}
		},
		"foreign content": func(t *testing.T, _ CredentialFiles, path string) {
			chmod(t, path, 0o600)
			if err := os.WriteFile(path, []byte(domain.ComposeToken(strings.Repeat("a", 64), "b3RoZXI")), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"fifo": func(t *testing.T, _ CredentialFiles, path string) {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := unix.Mkfifo(path, 0o600); err != nil {
				t.Skipf("mkfifo unavailable: %v", err)
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			files, key, token := credentialFixture(t)
			path, err := files.Write(context.Background(), key, token)
			if err != nil {
				t.Fatal(err)
			}
			mutate(t, files, path)
			if _, _, err := files.Read(context.Background(), path); err == nil {
				t.Fatal("unsafe credential accepted")
			}
		})
	}
}

func TestCredentialFilesHonorCancellation(t *testing.T) {
	files, key, token := credentialFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := files.Write(ctx, key, token); err == nil {
		t.Fatal("canceled write succeeded")
	}
	if _, err := os.Stat(filepath.Join(files.StateDir, "mcp-http")); !os.IsNotExist(err) {
		t.Fatalf("canceled write touched state: %v", err)
	}
}

func chmod(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func replaceWithSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
