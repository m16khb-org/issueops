//go:build darwin || linux

package cmux

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReadPromptUsesOneNoFollowHandleAndRejectsUnsafeBytes(t *testing.T) {
	t.Run("safe regular file", func(t *testing.T) {
		root := canonicalTempDir(t)
		path := filepath.Join(root, "prompt")
		value := []byte("sealed prompt\n")
		if err := os.WriteFile(path, value, 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := ReadPrompt(root, path, digestBytes(value))
		if err != nil || string(got) != string(value) {
			t.Fatalf("prompt=%q err=%v", got, err)
		}
	})

	t.Run("leaf symlink", func(t *testing.T) {
		root := canonicalTempDir(t)
		target := filepath.Join(root, "target")
		if err := os.WriteFile(target, []byte("prompt"), 0o600); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, "prompt")
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPrompt(root, path, digestBytes([]byte("prompt"))); err == nil {
			t.Fatal("symlink prompt accepted")
		}
	})

	t.Run("symlinked parent", func(t *testing.T) {
		root := canonicalTempDir(t)
		external := canonicalTempDir(t)
		if err := os.WriteFile(filepath.Join(external, "prompt"), []byte("prompt"), 0o600); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(root, "linked")
		if err := os.Symlink(external, link); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPrompt(root, filepath.Join(link, "prompt"), digestBytes([]byte("prompt"))); err == nil {
			t.Fatal("symlinked prompt parent accepted")
		}
	})

	t.Run("oversize", func(t *testing.T) {
		root := canonicalTempDir(t)
		path := filepath.Join(root, "prompt")
		value := []byte(strings.Repeat("x", MaximumPromptBytes+1))
		if err := os.WriteFile(path, value, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPrompt(root, path, digestBytes(value)); err == nil {
			t.Fatal("oversized prompt accepted")
		}
	})

	t.Run("unsafe mode", func(t *testing.T) {
		root := canonicalTempDir(t)
		path := filepath.Join(root, "prompt")
		value := []byte("sealed prompt")
		if err := os.WriteFile(path, value, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPrompt(root, path, digestBytes(value)); err == nil {
			t.Fatal("non-0600 prompt accepted")
		}
	})

	t.Run("NUL", func(t *testing.T) {
		root := canonicalTempDir(t)
		path := filepath.Join(root, "prompt")
		value := []byte("sealed\x00prompt")
		if err := os.WriteFile(path, value, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadPrompt(root, path, digestBytes(value)); err == nil || !strings.Contains(err.Error(), "NUL") {
			t.Fatalf("NUL prompt error=%v", err)
		}
	})
}

func TestReadPromptUsesPortableSingleArgumentBoundary(t *testing.T) {
	const promptLimit = 64 << 10
	root := canonicalTempDir(t)
	for _, test := range []struct {
		name    string
		size    int
		wantErr bool
	}{
		{name: "maximum", size: promptLimit},
		{name: "maximum plus one", size: promptLimit + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, strings.ReplaceAll(test.name, " ", "-"))
			value := []byte(strings.Repeat("p", test.size))
			if err := os.WriteFile(path, value, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadPrompt(root, path, digestBytes(value))
			if test.wantErr {
				if err == nil {
					t.Fatalf("prompt size %d accepted", test.size)
				}
				return
			}
			if err != nil || len(got) != test.size {
				t.Fatalf("prompt size %d: got=%d err=%v", test.size, len(got), err)
			}
		})
	}
}

func TestValidatePromptLeafStatRequiresEffectiveUID(t *testing.T) {
	stat := unix.Stat_t{Mode: unix.S_IFREG | 0o600, Size: 1, Uid: uint32(os.Geteuid())}
	if err := validatePromptLeafStat(stat, uint32(os.Geteuid())); err != nil {
		t.Fatalf("current-owner leaf rejected: %v", err)
	}
	before := stat
	stat.Uid++
	if err := validatePromptLeafStat(stat, uint32(os.Geteuid())); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("wrong-owner leaf accepted: %v", err)
	}
	if samePromptStat(before, stat) {
		t.Fatal("prompt owner change was treated as stable")
	}
}

func TestReadPromptRejectsNamespaceReplacementAfterOpen(t *testing.T) {
	root := canonicalTempDir(t)
	path := filepath.Join(root, "prompt")
	original := []byte("original sealed prompt")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readPromptPlatform(root, path, digestBytes(original), func() {
		if renameErr := os.Rename(path, filepath.Join(root, "opened-prompt")); renameErr != nil {
			t.Fatal(renameErr)
		}
		if writeErr := os.WriteFile(path, []byte("replacement prompt"), 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "namespace changed") {
		t.Fatalf("replacement prompt error=%v", err)
	}
}
