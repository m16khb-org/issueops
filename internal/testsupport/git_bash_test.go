package testsupport

import (
	"path/filepath"
	"strings"
	"testing"
)

func existsOnly(paths ...string) func(string) bool {
	set := map[string]bool{}
	for _, path := range paths {
		set[path] = true
	}
	return func(path string) bool { return set[path] }
}

func TestFindGitBashWalksUpFromUcrt64Git(t *testing.T) {
	git := AbsPath("Program Files", "Git", "ucrt64", "bin", "git.exe")
	want := AbsPath("Program Files", "Git", "bin", "bash.exe")

	got, err := findGitBash(git, existsOnly(want), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("bash = %q, want %q", got, want)
	}
}

func TestFindGitBashKeepsCmdLayout(t *testing.T) {
	git := AbsPath("Program Files", "Git", "cmd", "git.exe")
	want := AbsPath("Program Files", "Git", "bin", "bash.exe")

	got, err := findGitBash(git, existsOnly(want), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("bash = %q, want %q", got, want)
	}
}

func TestFindGitBashFallsBackToProgramFiles(t *testing.T) {
	git := AbsPath("tools", "a", "b", "c", "git.exe")
	programFiles := AbsPath("Program Files")
	want := filepath.Join(programFiles, "Git", "bin", "bash.exe")

	got, err := findGitBash(git, existsOnly(want), []string{AbsPath("Program Files (x86)"), programFiles})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("bash = %q, want %q", got, want)
	}
}

func TestFindGitBashStopsAfterThreeRoots(t *testing.T) {
	// The roots tried are a\b\c\d, a\b\c and a\b; a\bin\bash.exe is the fourth.
	git := AbsPath("a", "b", "c", "d", "e", "git.exe")
	tooHigh := AbsPath("a", "bin", "bash.exe")
	if _, err := findGitBash(git, existsOnly(tooHigh, AbsPath("bin", "bash.exe")), nil); err == nil {
		t.Fatal("findGitBash looked above the third root")
	}

	third := AbsPath("a", "b", "usr", "bin", "bash.exe")
	got, err := findGitBash(git, existsOnly(third, tooHigh), nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != third {
		t.Fatalf("bash = %q, want %q", got, third)
	}
}

func TestFindGitBashReportsTheGitPathWhenMissing(t *testing.T) {
	git := AbsPath("Program Files", "Git", "cmd", "git.exe")

	got, err := findGitBash(git, existsOnly(), []string{AbsPath("Program Files")})
	if err == nil {
		t.Fatalf("bash = %q, want an error", got)
	}
	if !strings.Contains(err.Error(), "not found next to "+git) {
		t.Fatalf("error = %q, want it to name %q", err, git)
	}
}

func TestFindGitBashPrefersTheSameInstallThenProgramFiles(t *testing.T) {
	programFiles := AbsPath("Program Files")
	standard := filepath.Join(programFiles, "Git", "bin", "bash.exe")
	ownBash := AbsPath("Tools", "Git", "bin", "bash.exe")
	cases := []struct {
		name   string
		git    string
		exists []string
		want   string
	}{
		{
			name:   "cmd layout outside Program Files without its own bash prefers the standard install over a second-root bash",
			git:    AbsPath("Tools", "Git", "cmd", "git.exe"),
			exists: []string{standard, AbsPath("Tools", "bin", "bash.exe"), AbsPath("bin", "bash.exe")},
			want:   standard,
		},
		{
			name:   "ucrt64 git prefers the bash of its own install",
			git:    AbsPath("Tools", "Git", "ucrt64", "bin", "git.exe"),
			exists: []string{standard, ownBash},
			want:   ownBash,
		},
		{
			name:   "mingw64 git prefers the bash of its own install",
			git:    AbsPath("Tools", "Git", "mingw64", "bin", "git.exe"),
			exists: []string{standard, ownBash},
			want:   ownBash,
		},
		{
			name:   "subsystem directory names match regardless of case",
			git:    AbsPath("Tools", "Git", "MINGW64", "bin", "git.exe"),
			exists: []string{standard, ownBash},
			want:   ownBash,
		},
		{
			name:   "deep non-subsystem path prefers the standard install over the third root",
			git:    AbsPath("a", "b", "c", "d", "e", "git.exe"),
			exists: []string{standard, AbsPath("a", "b", "bin", "bash.exe")},
			want:   standard,
		},
		{
			name:   "a bash in the cmd layout's own install wins over the standard install",
			git:    AbsPath("Tools", "Git", "cmd", "git.exe"),
			exists: []string{standard, ownBash},
			want:   ownBash,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := findGitBash(tc.git, existsOnly(tc.exists...), []string{programFiles})
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("bash = %q, want %q", got, tc.want)
			}
		})
	}
}
