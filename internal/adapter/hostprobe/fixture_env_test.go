package hostprobe

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/testsupport"
)

// sealedGetenv is a Dependencies.Getenv that sees only values, so no ambient
// variable (the developer's HOME/USERPROFILE, CODEX_HOME, child-smoke switches)
// reaches a runner and a test never reads real credentials.
func sealedGetenv(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

// sealedHome is sealedGetenv with only HOME set to an (empty) home directory.
func sealedHome(home string) func(string) string {
	return sealedGetenv(map[string]string{"HOME": home})
}

// fixturePath is a synthetic absolute path for fixtures never touched on disk
// ("/opt/bin" on Unix, "C:\opt\bin" on Windows).
func fixturePath(parts ...string) string { return testsupport.AbsPath(parts...) }

// filePerm is the Unix-style permission of path: the mode bits on Unix, and on
// Windows the rendering osfile derives from the owner and the DACL.
func filePerm(path string, info fs.FileInfo) fs.FileMode { return osfile.Perm(path, info) }

// makePrivate sets path to the private mode (0600 or 0700), also dropping any
// ACL testsupport.MakePermissive added on Windows.
func makePrivate(t testing.TB, path string, mode fs.FileMode) {
	t.Helper()
	if err := osfile.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

// makePermissive gives path a non-private mode such as 0644 or 0755: chmod on
// Unix, an extra readable-by-Users ACL entry on Windows.
func makePermissive(t testing.TB, path string, mode fs.FileMode) {
	t.Helper()
	testsupport.MakePermissive(t, path, mode)
}

// dataFilePerm is filePerm for a data file: on Windows osfile renders a file
// whose extension is in PATHEXT (.js here) as executable, so the owner execute
// bit is not part of its mode.
func dataFilePerm(path string, info fs.FileInfo) fs.FileMode {
	perm := osfile.Perm(path, info)
	if runtime.GOOS == "windows" {
		perm &^= 0o100
	}
	return perm
}

// isolatedHomeEnvNames are the variables, besides HOME, an isolated host env
// points into the episode: none on Unix, USERPROFILE/APPDATA/LOCALAPPDATA on
// Windows.
func isolatedHomeEnvNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"APPDATA", "LOCALAPPDATA", "USERPROFILE"}
	}
	return nil
}

// isolatedEnvWithHome is the sorted env an isolated host receives: entries plus
// HOME=home and, on Windows, the profile variables under home.
func isolatedEnvWithHome(home string, entries ...string) []string {
	env := append([]string{"HOME=" + home}, entries...)
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+home,
			"APPDATA="+filepath.Join(home, "AppData", "Roaming"),
			"LOCALAPPDATA="+filepath.Join(home, "AppData", "Local"))
	}
	sort.Strings(env)
	return env
}

// assertIsolatedHome checks env points every home variable into home: HOME on
// every OS, and on Windows USERPROFILE == home with APPDATA and LOCALAPPDATA
// under it, all private directories once the episode is prepared.
func assertIsolatedHome(t testing.TB, env []string, home string) {
	t.Helper()
	value := func(name string) string {
		for _, entry := range env {
			if after, ok := strings.CutPrefix(entry, name+"="); ok {
				return after
			}
		}
		return ""
	}
	if value("HOME") != home {
		t.Fatalf("HOME = %q, want %q", value("HOME"), home)
	}
	if runtime.GOOS != "windows" {
		if value("USERPROFILE") != "" || value("APPDATA") != "" || value("LOCALAPPDATA") != "" {
			t.Fatalf("Unix isolated env must not carry Windows profile variables: %q", env)
		}
		return
	}
	if value("USERPROFILE") != home {
		t.Fatalf("USERPROFILE = %q, want the isolated home %q", value("USERPROFILE"), home)
	}
	for name, dir := range map[string]string{"APPDATA": filepath.Join(home, "AppData", "Roaming"), "LOCALAPPDATA": filepath.Join(home, "AppData", "Local")} {
		if value(name) != dir {
			t.Fatalf("%s = %q, want %q", name, value(name), dir)
		}
	}
}

// assertIsolatedHomeDirs checks the Windows APPDATA and LOCALAPPDATA
// directories exist and are private while the episode is live.
func assertIsolatedHomeDirs(t testing.TB, home string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}
	for _, dir := range []string{filepath.Join(home, "AppData", "Roaming"), filepath.Join(home, "AppData", "Local")} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() || filePerm(dir, info) != 0o700 {
			t.Fatalf("isolated dir %s info=%v err=%v", dir, info, err)
		}
	}
}
