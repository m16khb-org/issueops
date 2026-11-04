//go:build windows

package osfile

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/sys/windows"
)

func grantUsersRead(t *testing.T, path string) {
	t.Helper()
	users, err := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	current, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(users)
	dacl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_READ,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_GROUP,
			TrusteeValue: windows.TrusteeValueFromSID(users),
		},
	}}, current)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func permOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return Perm(path, info)
}

func TestPermReportsOwnerOnlyObjectsAsPrivate(t *testing.T) {
	// The temp root may inherit ACEs for other principals (for example a
	// sandbox group), so establish an owner-only root first.
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "token")
	exe := filepath.Join(dir, "tool.exe")
	for _, path := range []string{file, exe} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := permOf(t, dir); got != 0o700 {
		t.Fatalf("directory perm = %o, want 700", got)
	}
	if got := permOf(t, file); got != 0o600 {
		t.Fatalf("file perm = %o, want 600", got)
	}
	if got := permOf(t, exe); got != 0o700 {
		t.Fatalf("executable perm = %o, want 700", got)
	}
	if err := os.Chmod(file, 0o400); err != nil {
		t.Fatal(err)
	}
	if got := permOf(t, file); got != 0o400 {
		t.Fatalf("read-only perm = %o, want 400", got)
	}
}

func TestPermExposesAnotherPrincipalAndChmodRestoresOwnerOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	grantUsersRead(t, dir)
	if got := permOf(t, dir); got != 0o777 {
		t.Fatalf("shared directory perm = %o, want 777", got)
	}
	if err := Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := permOf(t, dir); got != 0o700 {
		t.Fatalf("restricted directory perm = %o, want 700", got)
	}
	child := filepath.Join(dir, "child")
	if err := os.WriteFile(child, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := permOf(t, child); got != 0o600 {
		t.Fatalf("child of restricted directory perm = %o, want 600", got)
	}
	grantUsersRead(t, child)
	if got := permOf(t, child); got != 0o666 {
		t.Fatalf("shared file perm = %o, want 666", got)
	}
	if err := Chmod(child, 0o644); err != nil {
		t.Fatal(err)
	}
	// Group/other access follows the parent, which is owner-only here.
	if got := permOf(t, child); got != 0o600 {
		t.Fatalf("inherited file perm = %o, want 600", got)
	}
}

func TestIdentityMatchesPathAndOpenFileAndCountsLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	byPath, ok := PathIdentity(path, info)
	if !ok || byPath.Inode == 0 || byPath.Links != 1 {
		t.Fatalf("path identity = %+v, %v", byPath, ok)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	byFile, ok := FileIdentity(file, info)
	if !ok || byFile != byPath {
		t.Fatalf("file identity = %+v, want %+v", byFile, byPath)
	}
	if err := os.Link(path, filepath.Join(dir, "b")); err != nil {
		t.Fatal(err)
	}
	linked, ok := PathIdentity(path, info)
	if !ok || linked.Links != 2 || linked.Inode != byPath.Inode {
		t.Fatalf("linked identity = %+v", linked)
	}
	other := filepath.Join(dir, "c")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if different, _ := PathIdentity(other, info); different.Inode == byPath.Inode {
		t.Fatal("distinct files share an identity")
	}
}
